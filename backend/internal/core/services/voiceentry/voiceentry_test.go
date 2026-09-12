package voiceentry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubs stands in for both providers and records what each was sent.
type stubs struct {
	svc            *Service
	transcribeBody []byte
	transcribeAuth string
	llmAuth        string
	llmPrompt      string
}

func newStubs(t *testing.T, transcript, reply string, transcribeStatus, llmStatus int) *stubs {
	t.Helper()

	s := &stubs{}

	speech := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.transcribeBody, _ = io.ReadAll(r.Body)
		s.transcribeAuth = r.Header.Get("authorization")

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(transcribeStatus)
		_, _ = w.Write([]byte(transcript))
	}))
	t.Cleanup(speech.Close)

	reading := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.llmPrompt = string(body)
		s.llmAuth = r.Header.Get("x-api-key")

		w.Header().Set("content-type", "application/json")
		w.WriteHeader(llmStatus)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(reading.Close)

	s.svc = New(true, "server-speech", "server-reading", "")
	s.svc.Endpoint = speech.URL
	s.svc.llm.Endpoint = reading.URL

	return s
}

func heard(text string) string {
	payload, _ := json.Marshal(map[string]string{"text": text})
	return string(payload)
}

func said(fields string) string {
	payload, _ := json.Marshal(map[string]any{
		"content": []map[string]string{{"type": "text", "text": fields}},
	})
	return string(payload)
}

func TestDraft_TurnsSpeechIntoFields(t *testing.T) {
	s := newStubs(t,
		heard("drei packungen warta a a batterien"),
		said(`{"name":"Varta AA Batterien","quantity":3,"description":"Liegen in der Werkstattkiste."}`),
		http.StatusOK, http.StatusOK)

	draft, err := s.svc.Draft(context.Background(), "group-speech", "group-reading", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)

	assert.Equal(t, "Varta AA Batterien", draft.Name)
	assert.Equal(t, 3, draft.Quantity)
	assert.Equal(t, "Liegen in der Werkstattkiste.", draft.Description)

	// The transcript comes back too, so a misunderstanding is visible rather
	// than buried inside a tidied-up name.
	assert.Equal(t, "drei packungen warta a a batterien", draft.Transcript)
}

func TestDraft_UsesTheGroupKeysInPreferenceToTheServerOnes(t *testing.T) {
	s := newStubs(t, heard("etwas"), said(`{"name":"Etwas","quantity":1,"description":""}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "group-speech", "group-reading", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)

	assert.Equal(t, "Bearer group-speech", s.transcribeAuth)
	assert.Equal(t, "group-reading", s.llmAuth)
}

func TestDraft_FallsBackToTheServerKeys(t *testing.T) {
	s := newStubs(t, heard("etwas"), said(`{"name":"Etwas","quantity":1,"description":""}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "  ", "", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)

	assert.Equal(t, "Bearer server-speech", s.transcribeAuth)
	assert.Equal(t, "server-reading", s.llmAuth)
}

func TestDraft_SendsTheLanguageHint(t *testing.T) {
	s := newStubs(t, heard("etwas"), said(`{"name":"Etwas","quantity":1,"description":""}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de-DE")
	require.NoError(t, err)

	// Brand names are what this has to get right, and the hint is what makes
	// the difference on them.
	assert.Contains(t, string(s.transcribeBody), "de")
	assert.Contains(t, string(s.transcribeBody), "whisper-1")
}

func TestDraft_DisabledSendsNothing(t *testing.T) {
	s := newStubs(t, heard("x"), said(`{"name":"X","quantity":1}`), http.StatusOK, http.StatusOK)
	s.svc = New(false, "k", "k", "")

	_, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	assert.ErrorIs(t, err, ErrDisabled)
}

func TestDraft_MissingKeysAreReportedSeparately(t *testing.T) {
	s := newStubs(t, heard("x"), said(`{"name":"X","quantity":1}`), http.StatusOK, http.StatusOK)
	s.svc.transcribeKey = ""
	s.svc.interpretKey = ""

	_, err := s.svc.Draft(context.Background(), "", "reading", []byte("audio"), "clip.webm", "de")
	assert.ErrorIs(t, err, ErrNoKey)

	_, err = s.svc.Draft(context.Background(), "speech", "", []byte("audio"), "clip.webm", "de")
	assert.ErrorIs(t, err, ErrNoInterpretKey, "the two steps need different keys, so say which one is missing")
}

func TestDraft_OversizedRecordingIsRefusedBeforeItIsSent(t *testing.T) {
	s := newStubs(t, heard("x"), said(`{"name":"X","quantity":1}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "a", "b", make([]byte, MaxAudioBytes+1), "clip.webm", "de")
	assert.ErrorIs(t, err, ErrTooLarge)
	assert.Empty(t, s.transcribeBody, "nothing should have been sent")
}

func TestDraft_SilenceIsNotAnItem(t *testing.T) {
	s := newStubs(t, heard("   "), said(`{"name":"X","quantity":1}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	assert.ErrorIs(t, err, ErrEmpty)
	assert.Empty(t, s.llmPrompt, "the tidying step should not run on nothing")
}

func TestDraft_TranscriptionFailureIsReported(t *testing.T) {
	s := newStubs(t, `{"error":{"message":"invalid api key"}}`, said(`{}`), http.StatusUnauthorized, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid api key")
}

func TestDraft_ATidyingFailureStillKeepsWhatWasSaid(t *testing.T) {
	// Losing the recording because the second step broke would be the worse
	// outcome: the words are the part that cannot be recovered.
	s := newStubs(t, heard("zwei dosen mais"), `{"error":{"message":"overloaded"}}`, http.StatusOK, http.StatusInternalServerError)

	draft, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)

	assert.Equal(t, "zwei dosen mais", draft.Transcript)
	assert.Equal(t, "zwei dosen mais", draft.Name)
	assert.Equal(t, 1, draft.Quantity)
}

func TestDraft_QuantityIsNeverBelowOne(t *testing.T) {
	s := newStubs(t, heard("mais"), said(`{"name":"Mais","quantity":0,"description":""}`), http.StatusOK, http.StatusOK)

	draft, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)
	assert.Equal(t, 1, draft.Quantity)
}

func TestSafeFilename(t *testing.T) {
	// The extension is how the service works out the container format; the rest
	// of a browser-supplied name is not worth forwarding.
	assert.Equal(t, "audio.webm", safeFilename("../../etc/passwd"))
	assert.Equal(t, "audio.webm", safeFilename(""))
	assert.Equal(t, "audio.mp4", safeFilename("recording.mp4"))
	assert.Equal(t, "audio.ogg", safeFilename("x.ogg"))
	assert.Equal(t, "audio.wav", safeFilename("x.wav"))
}

func TestNormaliseLanguage(t *testing.T) {
	assert.Equal(t, "de", normaliseLanguage("de"))
	assert.Equal(t, "de", normaliseLanguage("de-DE"))
	assert.Equal(t, "pt", normaliseLanguage("pt_BR"))
	assert.Equal(t, "", normaliseLanguage(""))
	assert.Equal(t, "", normaliseLanguage("deutsch"))
	assert.Equal(t, "", normaliseLanguage("d3"))
}

func TestDraft_ToleratesAFencedAnswer(t *testing.T) {
	fenced := "```json\n{\"name\":\"Mais\",\"quantity\":2,\"description\":\"\"}\n```"
	s := newStubs(t, heard("zwei dosen mais"), said(fenced), http.StatusOK, http.StatusOK)

	draft, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)
	assert.Equal(t, "Mais", draft.Name)
	assert.Equal(t, 2, draft.Quantity)
}

func TestDraft_TheTranscriptIsWhatGoesToTheModel(t *testing.T) {
	s := newStubs(t, heard("drei packungen batterien"),
		said(`{"name":"Batterien","quantity":3,"description":""}`), http.StatusOK, http.StatusOK)

	_, err := s.svc.Draft(context.Background(), "a", "b", []byte("audio"), "clip.webm", "de")
	require.NoError(t, err)

	assert.True(t, strings.Contains(s.llmPrompt, "drei packungen batterien"))
	// The audio itself must not be forwarded to the second provider.
	assert.False(t, strings.Contains(s.llmPrompt, "audio"))
}
