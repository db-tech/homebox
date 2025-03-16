package main

import (
	"github.com/go-chi/chi/v5"
	"github.com/hay-kot/httpkit/errchain"
	v1 "github.com/sysadminsmedia/homebox/backend/app/api/handlers/v1"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/authroles"
)

// setupAdminRoutes configures all admin-only routes
func (a *app) setupAdminRoutes(r chi.Router, chain *errchain.ErrChain, v1Ctrl *v1.V1Controller) {
	// Admin middleware requires superuser status
	adminMW := []errchain.Middleware{
		a.mwAuthToken,
		a.mwRoles(RoleModeOr, authroles.RoleUser.String()),
		a.mwRequireSuperuser,
	}

	// Admin user management routes - explicit routes for clarity
	r.Get("/admin/users", chain.ToHandlerFunc(v1Ctrl.HandleAdminGetAllUsers(), adminMW...))
	r.Post("/admin/users", chain.ToHandlerFunc(v1Ctrl.HandleAdminCreateUser(), adminMW...))
	r.Put("/admin/users/{id}", chain.ToHandlerFunc(v1Ctrl.HandleAdminUpdateUser(), adminMW...))
	r.Delete("/admin/users/{id}", chain.ToHandlerFunc(v1Ctrl.HandleAdminDeleteUser(), adminMW...))
}