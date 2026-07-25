package service

import (
	"errors"

	"mikvoc/internal/authn"
	"mikvoc/internal/core"
	"mikvoc/internal/repository"
)

type AuthService struct {
	admins repository.AdminRepo
}

func NewAuth(admins repository.AdminRepo) *AuthService {
	return &AuthService{admins: admins}
}

func (s *AuthService) Login(username, password string) (*core.Admin, error) {
	admin, err := s.admins.GetAdminByUsername(username)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			dbUser, dbPassHash := s.admins.GetAdmin()
			if dbUser == "" || username != dbUser || !authn.VerifyPassword(dbPassHash, password) {
				return nil, core.ErrUnauthorized
			}
			if !authn.IsBcryptHash(dbPassHash) {
				hashed, herr := authn.HashPassword(password)
				if herr == nil {
					_ = s.admins.SetAdmin(dbUser, hashed)
					dbPassHash = hashed
				}
			}
			return &core.Admin{
				Username:     dbUser,
				PasswordHash: dbPassHash,
				Role:         core.RoleOwner,
			}, nil
		}
		return nil, err
	}

	if !authn.VerifyPassword(admin.PasswordHash, password) {
		return nil, core.ErrUnauthorized
	}

	if !authn.IsBcryptHash(admin.PasswordHash) {
		hashed, herr := authn.HashPassword(password)
		if herr == nil {
			_ = s.admins.UpdateAdmin(admin.ID, admin.Username, hashed, admin.Role)
			admin.PasswordHash = hashed
		}
	}

	if admin.Role == "" {
		admin.Role = core.RoleOwner
	}
	return admin, nil
}

func (s *AuthService) SetCredentials(username, newPassword string) error {
	hash := ""
	if newPassword != "" {
		h, err := authn.HashPassword(newPassword)
		if err != nil {
			return err
		}
		hash = h
	} else {
		_, existing := s.admins.GetAdmin()
		hash = existing
	}
	return s.admins.SetAdmin(username, hash)
}
