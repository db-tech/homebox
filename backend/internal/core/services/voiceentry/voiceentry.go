// Package voiceentry turns a few spoken words into a filled-in item form.
//
// It exists for the things a scanner cannot help with: everything without a
// barcode. Saying "drei Packungen Varta AA Batterien" is faster than typing it
// on a phone held in one hand, which is how this inventory actually gets filled.
//
// Two steps with two providers. The audio goes to OpenAI to be transcribed,
// because Anthropic's models do not take audio at all. The transcript then goes
// to Anthropic to be turned into field values, using the key already configured
// for the meal suggestions.
//
// Nothing is ever created from this. The result fills a form that somebody
// confirms, which is not politeness: German product names are exactly what
// speech recognition gets wrong, and a wrong name saved silently is worse than
// one typed slowly.
package voiceentry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/sysadminsmedia/homebox/backend/internal/core/services/llm"
)

const defaultTranscribeEndpoint = "https://api.openai.com/v1/audio/transcriptions"

// transcribeModel is OpenAI's Whisper endpoint model.
const transcribeModel = "whisper-1"

// MaxAudioBytes caps what will be forwarded. A spoken item description is a few
// seconds; anything much larger is a mistake or an accident, and both are
// better refused here than paid for.
const MaxAudioBytes = 8 << 20 // 8 MiB

var (
	// ErrDisabled is returned when the feature is switched off by config.
	ErrDisabled = errors.New("voice entry is disabled")
	// ErrNoKey is returned when no transcription key is configured.
	ErrNoKey = errors.New("voice entry needs an OpenAI API key")
	// ErrNoInterpretKey is returned when there is no key for the tidying step.
	ErrNoInterpretKey = errors.New("voice entry needs an Anthropic API key to read the transcript")
	// ErrTooLarge is returned for an oversized recording.
	ErrTooLarge = errors.New("the recording is too long")
	// ErrEmpty is returned when nothing intelligible was said.
	ErrEmpty = errors.New("nothing was understood")
)

// Draft is a filled-in form waiting to be confirmed.
type Draft struct {
	// Transcript is what was heard, shown alongside the fields so a
	// misunderstanding is visible rather than buried in a tidied-up name.
	Transcript  string `json:"transcript"`
	Name        string `json:"name"`
	Quantity    int    `json:"quantity"`
	Description string `json:"description"`
}

type Service struct {
	enabled bool
	// transcribeKey and interpretKey are server-wide fallbacks, used when a
	// group has none of its own.
	transcribeKey string
	interpretKey  string

	// Endpoint is a field so tests can point it at a stub.
	Endpoint string
	http     *http.Client
	llm      *llm.Client
}

func New(enabled bool, transcribeKey, interpretKey, model string) *Service {
	return &Service{
		enabled:       enabled,
		transcribeKey: strings.TrimSpace(transcribeKey),
		interpretKey:  strings.TrimSpace(interpretKey),
		Endpoint:      defaultTranscribeEndpoint,
		// Somebody is watching a spinner having just spoken, so this is not the
		// place to wait a minute - but transcription is slower than a text call.
		http: &http.Client{Timeout: 45 * time.Second},
		llm:  llm.NewClient(model, 30*time.Second),
	}
}

// Enabled says whether the server permits voice entry. It says nothing about
// whether a key has been configured - that is per group.
func (s *Service) Enabled() bool {
	return s != nil && s.enabled
}

const systemPrompt = `You turn a spoken sentence about a household item into form values.

The speaker is standing in front of a shelf saying what they are putting into their inventory. The text you get is a speech-to-text transcript, so it may have odd spellings of brand names, no punctuation, and filler words.

Return:
- "name": the item, in the spelling and capitalisation a shelf label would use. Correct an obviously mis-heard brand name to the real one. Do not include the quantity in the name.
- "quantity": how many, as a whole number. Use 1 when no number is said.
- "description": one short sentence of useful detail actually present in what was said - size, colour, variant, where it belongs. Empty string if nothing was said beyond the name.

Rules:
- Invent nothing. If the speaker did not say what size or colour it is, the description does not mention one.
- Do not state facts about the product you were not told. You cannot look anything up, so anything you add is a guess and guesses do not belong in an inventory.
- Reply in the language the transcript is in.

Reply with JSON only, no prose and no code fence:
{"name":"...","quantity":1,"description":"..."}`

// Draft transcribes a recording and reads it into form values.
//
// transcribeKey and interpretKey come from the group; the server-wide ones are
// used only when a group has none.
func (s *Service) Draft(ctx context.Context, transcribeKey, interpretKey string, audio []byte, filename, language string) (Draft, error) {
	if s == nil || !s.enabled {
		return Draft{}, ErrDisabled
	}

	speech := firstNonEmpty(transcribeKey, s.transcribeKey)
	reading := firstNonEmpty(interpretKey, s.interpretKey)

	switch {
	case speech == "":
		return Draft{}, ErrNoKey
	case reading == "":
		return Draft{}, ErrNoInterpretKey
	case len(audio) == 0:
		return Draft{}, ErrEmpty
	case len(audio) > MaxAudioBytes:
		return Draft{}, ErrTooLarge
	}

	transcript, err := s.transcribe(ctx, speech, audio, filename, language)
	if err != nil {
		return Draft{}, err
	}

	transcript = strings.TrimSpace(transcript)
	if transcript == "" {
		return Draft{}, ErrEmpty
	}

	draft, err := s.interpret(ctx, reading, transcript)
	if err != nil {
		// The transcript on its own is still worth having: it goes into the
		// name field and can be tidied by hand. Losing the recording because
		// the second step failed would be the worse outcome.
		return Draft{Transcript: transcript, Name: transcript, Quantity: 1}, nil
	}

	draft.Transcript = transcript
	return draft, nil
}

func (s *Service) transcribe(ctx context.Context, apiKey string, audio []byte, filename, language string) (string, error) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)

	part, err := form.CreateFormFile("file", safeFilename(filename))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}

	fields := map[string]string{"model": transcribeModel, "response_format": "json"}
	// A language hint makes a real difference to German brand names, which are
	// the words this has to get right.
	if lang := normaliseLanguage(language); lang != "" {
		fields["language"] = lang
	}

	for name, value := range fields {
		if err := form.WriteField(name, value); err != nil {
			return "", err
		}
	}

	if err := form.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Endpoint, &body)
	if err != nil {
		return "", err
	}

	req.Header.Set("content-type", form.FormDataContentType())
	req.Header.Set("authorization", "Bearer "+apiKey)

	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	var parsed struct {
		Text  string `json:"text"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", fmt.Errorf("voice entry: unreadable transcription response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return "", fmt.Errorf("voice entry: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("voice entry: transcription failed with status %d", resp.StatusCode)
	}

	return parsed.Text, nil
}

func (s *Service) interpret(ctx context.Context, apiKey, transcript string) (Draft, error) {
	text, err := s.llm.Complete(ctx, apiKey, systemPrompt, transcript, 512)
	if err != nil {
		return Draft{}, err
	}

	var draft Draft
	if err := json.Unmarshal([]byte(llm.ExtractJSON(text)), &draft); err != nil {
		return Draft{}, fmt.Errorf("voice entry: could not read the fields: %w", err)
	}

	draft.Name = strings.TrimSpace(draft.Name)
	draft.Description = strings.TrimSpace(draft.Description)

	if draft.Name == "" {
		return Draft{}, ErrEmpty
	}
	if draft.Quantity < 1 {
		draft.Quantity = 1
	}

	return draft, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// safeFilename keeps the extension, which is how the transcription service
// works out the container format, and throws the rest away.
func safeFilename(name string) string {
	switch {
	case strings.HasSuffix(name, ".mp4"), strings.HasSuffix(name, ".m4a"):
		return "audio.mp4"
	case strings.HasSuffix(name, ".ogg"):
		return "audio.ogg"
	case strings.HasSuffix(name, ".wav"):
		return "audio.wav"
	case strings.HasSuffix(name, ".mp3"):
		return "audio.mp3"
	default:
		// What browsers record by default.
		return "audio.webm"
	}
}

// normaliseLanguage reduces a locale to the two-letter code the transcription
// service expects, and drops anything that is not one.
func normaliseLanguage(locale string) string {
	code := strings.ToLower(strings.TrimSpace(locale))
	if idx := strings.IndexAny(code, "-_"); idx > 0 {
		code = code[:idx]
	}

	if len(code) != 2 {
		return ""
	}

	for _, r := range code {
		if r < 'a' || r > 'z' {
			return ""
		}
	}

	return code
}
