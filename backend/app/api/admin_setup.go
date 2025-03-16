package main

import (
	"context"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/sysadminsmedia/homebox/backend/internal/data/repo"
	"github.com/sysadminsmedia/homebox/backend/pkgs/hasher"
)

// setupAdminUser checks for environment variables and creates an admin user if configured
func (a *app) setupAdminUser() error {
	// Check if admin user creation is requested
	shouldCreateAdmin := os.Getenv("HBOX_ADMIN_CREATE")
	if strings.ToLower(shouldCreateAdmin) != "true" {
		log.Debug().Msg("Admin user creation not requested")
		return nil
	}

	adminName := os.Getenv("HBOX_ADMIN_NAME")
	adminEmail := os.Getenv("HBOX_ADMIN_EMAIL")
	adminPassword := os.Getenv("HBOX_ADMIN_PASSWORD")

	// Validate required fields
	if adminName == "" || adminEmail == "" || adminPassword == "" {
		log.Warn().Msg("Admin user creation requested but missing required environment variables (HBOX_ADMIN_NAME, HBOX_ADMIN_EMAIL, HBOX_ADMIN_PASSWORD)")
		return nil
	}

	ctx := context.Background()

	// Check if user already exists
	existingUser, err := a.repos.Users.GetOneEmail(ctx, adminEmail)
	if err == nil {
		log.Info().Msgf("Admin user %s already exists", adminEmail)
		
		// If the user exists but is not a superuser, update them to be a superuser
		if !existingUser.IsSuperuser {
			log.Info().Msgf("Updating %s to be a superuser", adminEmail)
			updateData := repo.UserUpdate{
				Name:  adminName,
				Email: adminEmail,
			}
			err = a.repos.Users.Update(ctx, existingUser.ID, updateData)
			if err != nil {
				log.Err(err).Msg("Failed to update user data")
				return err
			}
			
			// Update the user to be a superuser
			err = a.repos.Users.SetSuperuser(ctx, existingUser.ID, true)
			if err != nil {
				log.Err(err).Msg("Failed to set user as superuser")
				return err
			}
			
			log.Info().Msgf("User %s is now a superuser", adminEmail)
		} else {
			log.Info().Msgf("User %s is already a superuser", adminEmail)
		}
		
		return nil
	}

	log.Info().Msgf("Creating admin user %s", adminEmail)

	// Create a new group for the admin
	group, err := a.repos.Groups.GroupCreate(ctx, "Admin Group")
	if err != nil {
		log.Err(err).Msg("Failed to create admin group")
		return err
	}

	// Create the admin user
	hashedPassword, _ := hasher.HashPassword(adminPassword)
	userCreate := repo.UserCreate{
		Name:        adminName,
		Email:       adminEmail,
		Password:    hashedPassword,
		IsSuperuser: true, // Make this user a superuser
		GroupID:     group.ID,
		IsOwner:     true,
	}

	user, err := a.repos.Users.Create(ctx, userCreate)
	if err != nil {
		log.Err(err).Msg("Failed to create admin user")
		return err
	}

	log.Info().Msgf("Admin user created successfully: %s (ID: %s)", user.Email, user.ID)

	// Create default labels and locations
	createDefaults(ctx, a.repos, user.GroupID)

	return nil
}

// createDefaults creates default labels and locations for a group
func createDefaults(ctx context.Context, repos *repo.AllRepos, groupID uuid.UUID) {
	log.Debug().Msg("Creating default labels")
	for _, label := range defaultLabels() {
		_, err := repos.Labels.Create(ctx, groupID, label)
		if err != nil {
			log.Err(err).Msg("Failed to create default label")
		}
	}

	log.Debug().Msg("Creating default locations")
	for _, location := range defaultLocations() {
		_, err := repos.Locations.Create(ctx, groupID, location)
		if err != nil {
			log.Err(err).Msg("Failed to create default location")
		}
	}
}

// defaultLabels returns a list of default labels
func defaultLabels() []repo.LabelCreate {
	return []repo.LabelCreate{
		{Name: "Electronics", Color: "#FF5733"},
		{Name: "Books", Color: "#33FF57"},
		{Name: "Clothing", Color: "#3357FF"},
		{Name: "Tools", Color: "#F3FF33"},
		{Name: "Furniture", Color: "#33FFF3"},
	}
}

// defaultLocations returns a list of default locations
func defaultLocations() []repo.LocationCreate {
	return []repo.LocationCreate{
		{Name: "Home"},
		{Name: "Garage"},
		{Name: "Storage"},
		{Name: "Office"},
	}
}
