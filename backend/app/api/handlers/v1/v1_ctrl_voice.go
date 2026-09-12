package v1

import (
	"errors"
	"io"
	"net/http"

	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/rs/zerolog/log"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/voiceentry"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
)

// HandleVoiceDraft godoc
//
//	@Summary		Turn a spoken description into item fields
//	@Description	Accepts a short audio recording and returns suggested values. Creates nothing.
//	@Tags			Items
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			audio		formData	file	true	"recording"
//	@Param			language	formData	string	false	"language hint, e.g. de"
//	@Success		200			{object}	voiceentry.Draft
//	@Router			/v1/items/voice-draft [POST]
//	@Security		Bearer
//
// Nothing is created here on purpose. The result fills a form somebody
// confirms: speech recognition mishears brand names, and an item saved from a
// misheard name is worse than one typed slowly.
func (ctrl *V1Controller) HandleVoiceDraft() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		auth := services.NewContext(r.Context())

		// Parsed with a hard ceiling so an oversized upload is refused by the
		// server rather than buffered in full and then rejected.
		err := r.ParseMultipartForm(voiceentry.MaxAudioBytes)
		if err != nil {
			return validate.NewRequestError(errors.New("could not read the recording"), http.StatusBadRequest)
		}

		file, header, err := r.FormFile("audio")
		if err != nil {
			return validate.NewRequestError(errors.New("no recording was sent"), http.StatusBadRequest)
		}
		defer func() { _ = file.Close() }()

		audio, err := io.ReadAll(io.LimitReader(file, voiceentry.MaxAudioBytes+1))
		if err != nil {
			return err
		}

		speech, err := ctrl.repo.Groups.VoiceAPIKey(auth, auth.GID)
		if err != nil {
			return err
		}

		reading, err := ctrl.repo.Groups.RecipesAPIKey(auth, auth.GID)
		if err != nil {
			return err
		}

		draft, err := ctrl.svc.VoiceEntry.Draft(
			auth, speech, reading, audio, header.Filename, r.FormValue("language"),
		)

		switch {
		case errors.Is(err, voiceentry.ErrDisabled),
			errors.Is(err, voiceentry.ErrNoKey),
			errors.Is(err, voiceentry.ErrNoInterpretKey):
			// The UI hides the button when no key is configured; this is the
			// guard for anything calling the route anyway.
			return validate.NewRequestError(err, http.StatusNotFound)
		case errors.Is(err, voiceentry.ErrTooLarge):
			return validate.NewRequestError(err, http.StatusRequestEntityTooLarge)
		case errors.Is(err, voiceentry.ErrEmpty):
			return validate.NewRequestError(err, http.StatusUnprocessableEntity)
		case err != nil:
			log.Warn().Err(err).Msg("voice draft failed")
			return validate.NewRequestError(err, http.StatusBadGateway)
		}

		return server.JSON(w, http.StatusOK, draft)
	}
}
