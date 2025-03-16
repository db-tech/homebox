package v1

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/hay-kot/httpkit/server"
	"github.com/rs/zerolog/log"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/validate"
)

// HandleUserRegistration godoc
//
//	@Summary	Register New User
//	@Tags		User
//	@Produce	json
//	@Param		payload	body	services.UserRegistration	true	"User Data"
//	@Success	204
//	@Router		/v1/users/register [Post]
func (ctrl *V1Controller) HandleUserRegistration() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		regData := services.UserRegistration{}

		if err := server.Decode(r, &regData); err != nil {
			log.Err(err).Msg("failed to decode user registration data")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		if !ctrl.allowRegistration && regData.GroupToken == "" {
			return validate.NewRequestError(fmt.Errorf("user registration disabled"), http.StatusForbidden)
		}

		_, err := ctrl.svc.User.RegisterUser(r.Context(), regData)
		if err != nil {
			log.Err(err).Msg("failed to register user")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusNoContent, nil)
	}
}

// HandleUserSelf godoc
//
//	@Summary	Get User Self
//	@Tags		User
//	@Produce	json
//	@Success	200	{object}	Wrapped{item=repo.UserOut}
//	@Router		/v1/users/self [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleUserSelf() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		token := services.UseTokenCtx(r.Context())
		usr, err := ctrl.svc.User.GetSelf(r.Context(), token)
		if usr.ID == uuid.Nil || err != nil {
			log.Err(err).Msg("failed to get user")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusOK, Wrap(usr))
	}
}

// HandleUserSelfUpdate godoc
//
//	@Summary	Update Account
//	@Tags		User
//	@Produce	json
//	@Param		payload	body		repo.UserUpdate	true	"User Data"
//	@Success	200		{object}	Wrapped{item=repo.UserUpdate}
//	@Router		/v1/users/self [PUT]
//	@Security	Bearer
func (ctrl *V1Controller) HandleUserSelfUpdate() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		updateData := repo.UserUpdate{}
		if err := server.Decode(r, &updateData); err != nil {
			log.Err(err).Msg("failed to decode user update data")
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		actor := services.UseUserCtx(r.Context())
		newData, err := ctrl.svc.User.UpdateSelf(r.Context(), actor.ID, updateData)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusOK, Wrap(newData))
	}
}

// HandleUserSelfDelete godoc
//
//	@Summary	Delete Account
//	@Tags		User
//	@Produce	json
//	@Success	204
//	@Router		/v1/users/self [DELETE]
//	@Security	Bearer
func (ctrl *V1Controller) HandleUserSelfDelete() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if ctrl.isDemo {
			return validate.NewRequestError(nil, http.StatusForbidden)
		}

		actor := services.UseUserCtx(r.Context())
		if err := ctrl.svc.User.DeleteSelf(r.Context(), actor.ID); err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusNoContent, nil)
	}
}

type (
	ChangePassword struct {
		Current string `json:"current,omitempty"`
		New     string `json:"new,omitempty"`
	}

	AdminUserCreate struct {
		Name        string    `json:"name"`
		Email       string    `json:"email"`
		Password    string    `json:"password"`
		IsSuperuser bool      `json:"isSuperuser"`
		GroupID     uuid.UUID `json:"groupID"`
	}

	// AdminUserUpdate extends UserUpdate to allow changing superuser status
	AdminUserUpdate struct {
		Name        string `json:"name"`
		Email       string `json:"email"`
		IsSuperuser bool   `json:"isSuperuser"`
	}
)

// HandleUserSelfChangePassword godoc
//
//	@Summary	Change Password
//	@Tags		User
//	@Success	204
//	@Param		payload	body	ChangePassword	true	"Password Payload"
//	@Router		/v1/users/change-password [PUT]
//	@Security	Bearer
func (ctrl *V1Controller) HandleUserSelfChangePassword() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if ctrl.isDemo {
			return validate.NewRequestError(nil, http.StatusForbidden)
		}

		var cp ChangePassword
		err := server.Decode(r, &cp)
		if err != nil {
			log.Err(err).Msg("user failed to change password")
		}

		ctx := services.NewContext(r.Context())

		ok := ctrl.svc.User.ChangePassword(ctx, cp.Current, cp.New)
		if !ok {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusNoContent, nil)
	}
}

// Admin API Handler Functions

// HandleAdminGetAllUsers godoc
//
//	@Summary	Get All Users (Admin Only)
//	@Tags		Admin
//	@Produce	json
//	@Success	200	{object}	Results[repo.UserOut]
//	@Router		/v1/admin/users [GET]
//	@Security	Bearer
func (ctrl *V1Controller) HandleAdminGetAllUsers() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		// User context is already verified as admin by middleware
		log.Info().Msg("Admin API: fetching all users")

		users, err := ctrl.repo.Users.GetAll(r.Context())
		if err != nil {
			log.Err(err).Msg("failed to get all users")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		// Get the current user from context for debugging
		actor := services.UseUserCtx(r.Context())
		if actor != nil {
			log.Info().Str("user_email", actor.Email).Bool("is_superuser", actor.IsSuperuser).Msg("Admin API: current user")
		} else {
			log.Warn().Msg("Admin API: no user in context")
		}

		log.Info().Int("user_count", len(users)).Msg("Admin API: returning users")

		// Explicitly set CORS headers for debugging
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Create a more explicit response object for debugging
		response := map[string]interface{}{
			"results": users,
			"count":   len(users),
		}

		return server.JSON(w, http.StatusOK, response)
	}
}

// HandleAdminCreateUser godoc
//
//	@Summary	Create New User (Admin Only)
//	@Tags		Admin
//	@Produce	json
//	@Param		payload	body	AdminUserCreate	true	"User Data"
//	@Success	201	{object}	Wrapped{item=repo.UserOut}
//	@Router		/v1/admin/users [POST]
//	@Security	Bearer
func (ctrl *V1Controller) HandleAdminCreateUser() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		// User context is already verified as admin by middleware
		userData := AdminUserCreate{}
		if err := server.Decode(r, &userData); err != nil {
			log.Err(err).Msg("failed to decode admin user create data")
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		actor := services.UseUserCtx(r.Context())
		if userData.GroupID == uuid.Nil {
			// Use the admin's group if none specified
			userData.GroupID = actor.GroupID
		}

		hashedPassword, err := ctrl.svc.User.HashPassword(userData.Password)
		if err != nil {
			log.Err(err).Msg("failed to hash password")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		user, err := ctrl.repo.Users.Create(r.Context(), repo.UserCreate{
			Name:        userData.Name,
			Email:       userData.Email,
			Password:    hashedPassword,
			IsSuperuser: userData.IsSuperuser,
			GroupID:     userData.GroupID,
			IsOwner:     false, // Admin-created users are not owners by default
		})
		if err != nil {
			log.Err(err).Msg("failed to create user")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusCreated, Wrap(user))
	}
}

// HandleAdminUpdateUser godoc
//
//	@Summary	Update User (Admin Only)
//	@Tags		Admin
//	@Produce	json
//	@Param		id		path	string			true	"User ID"
//	@Param		payload	body	repo.UserUpdate	true	"User Data"
//	@Success	200	{object}	Wrapped{item=repo.UserOut}
//	@Router		/v1/admin/users/{id} [PUT]
//	@Security	Bearer
func (ctrl *V1Controller) HandleAdminUpdateUser() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		// User context is already verified as admin by middleware
		userID := chi.URLParam(r, "id")
		if userID == "" {
			return validate.NewRequestError(fmt.Errorf("user id is required"), http.StatusBadRequest)
		}

		id, err := uuid.Parse(userID)
		if err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		adminUpdateData := AdminUserUpdate{}
		if err := server.Decode(r, &adminUpdateData); err != nil {
			log.Err(err).Msg("failed to decode admin user update data")
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		// First update basic user data
		updateData := repo.UserUpdate{
			Name:  adminUpdateData.Name,
			Email: adminUpdateData.Email,
		}

		err = ctrl.repo.Users.Update(r.Context(), id, updateData)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		// Then update superuser status
		err = ctrl.repo.Users.SetSuperuser(r.Context(), id, adminUpdateData.IsSuperuser)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		updatedUser, err := ctrl.repo.Users.GetOneID(r.Context(), id)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusOK, Wrap(updatedUser))
	}
}

// HandleAdminDeleteUser godoc
//
//	@Summary	Delete User (Admin Only)
//	@Tags		Admin
//	@Produce	json
//	@Param		id	path	string	true	"User ID"
//	@Success	204
//	@Router		/v1/admin/users/{id} [DELETE]
//	@Security	Bearer
func (ctrl *V1Controller) HandleAdminDeleteUser() errchain.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		// User context is already verified as admin by middleware
		if ctrl.isDemo {
			return validate.NewRequestError(nil, http.StatusForbidden)
		}

		userID := chi.URLParam(r, "id")
		if userID == "" {
			return validate.NewRequestError(fmt.Errorf("user id is required"), http.StatusBadRequest)
		}

		id, err := uuid.Parse(userID)
		if err != nil {
			return validate.NewRequestError(err, http.StatusBadRequest)
		}

		// Don't allow deleting your own account through admin API
		actor := services.UseUserCtx(r.Context())
		if actor.ID == id {
			return validate.NewRequestError(fmt.Errorf("cannot delete yourself through admin API"), http.StatusForbidden)
		}

		err = ctrl.repo.Users.Delete(r.Context(), id)
		if err != nil {
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.JSON(w, http.StatusNoContent, nil)
	}
}
