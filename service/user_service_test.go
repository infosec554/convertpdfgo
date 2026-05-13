package service

import (
	"context"
	"testing"

	"convertpdfgo/api/models"
	"convertpdfgo/pkg/logger"
	"convertpdfgo/storage"
)

// fakeUserStore captures UpdateRole/UpdateStatus calls so we can assert
// the validation gate without a real DB.
type fakeUserStore struct {
	storage.IUserStorage
	lastRole   string
	lastStatus string
}

func (f *fakeUserStore) UpdateRole(_ context.Context, _, role string) error {
	f.lastRole = role
	return nil
}

func (f *fakeUserStore) UpdateStatus(_ context.Context, _, status string) error {
	f.lastStatus = status
	return nil
}

// fakeStorageForUser plugs fakeUserStore into the wider IStorage so
// NewUserService can be constructed.
type fakeStorageForUser struct {
	storage.IStorage
	users storage.IUserStorage
}

func (s *fakeStorageForUser) User() storage.IUserStorage { return s.users }

func newTestUserService(us storage.IUserStorage) UserService {
	return &userService{stg: us, log: logger.NewWithLevel("test", "error")}
}

func TestUpdateRole_AcceptsTwoRoles(t *testing.T) {
	us := &fakeUserStore{}
	svc := newTestUserService(us)

	for _, role := range []string{"user", "admin"} {
		if err := svc.UpdateRole(context.Background(), "u-1", role); err != nil {
			t.Fatalf("role %q must be valid, got %v", role, err)
		}
		if us.lastRole != role {
			t.Fatalf("storage should see role %q, got %q", role, us.lastRole)
		}
	}
}

func TestUpdateRole_RejectsLegacyAndUnknown(t *testing.T) {
	svc := newTestUserService(&fakeUserStore{})

	// Old multi-tier roles must be rejected post-migration 0014.
	for _, bad := range []string{"superadmin", "moderator", "Admin", "", "root", "owner"} {
		if err := svc.UpdateRole(context.Background(), "u-1", bad); err == nil {
			t.Fatalf("role %q must be rejected", bad)
		}
	}
}

func TestUpdateStatus_Validation(t *testing.T) {
	us := &fakeUserStore{}
	svc := newTestUserService(us)

	for _, good := range []string{"active", "blocked", "deleted"} {
		if err := svc.UpdateStatus(context.Background(), "u-1", good); err != nil {
			t.Fatalf("status %q must be valid, got %v", good, err)
		}
	}
	for _, bad := range []string{"pending", "banned", "", "Active"} {
		if err := svc.UpdateStatus(context.Background(), "u-1", bad); err == nil {
			t.Fatalf("status %q must be rejected", bad)
		}
	}
}

// Compile-time: ensure models.User stays exported and Role is a string.
var _ = models.User{Role: "user"}
