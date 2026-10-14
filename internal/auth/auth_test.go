package auth

import (
	"path/filepath"
	"testing"

	"nstock/internal/config"
	"nstock/internal/database"
)

func testService(t *testing.T) *Service {
	t.Helper()
	cfg := config.Config{DBPath: filepath.Join(t.TempDir(), "auth.db")}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	return New(db)
}

func TestLoginRolesAndLastAdmin(t *testing.T) {
	svc := testService(t)
	adminLogin, err := svc.Login(UserAdmin, PassAdmin)
	if err != nil || adminLogin.Token == "" {
		t.Fatal(err)
	}
	admin, err := svc.Resolve("Bearer "+adminLogin.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.Allows("stock:user:create") || !admin.Allows("stock:order:create") {
		t.Fatal("admin should have user and order permissions")
	}

	analystLogin, err := svc.Login(UserAnalyst, PassAnalyst)
	if err != nil {
		t.Fatal(err)
	}
	analyst, err := svc.Resolve("Bearer "+analystLogin.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	if analyst.Allows("stock:order:create") || analyst.Allows("stock:user:view") {
		t.Fatal("analyst should be read-only")
	}
	if !analyst.Allows("stock:predict:execute") || !analyst.Allows("stock:quote:view") {
		t.Fatal("analyst should research")
	}

	if _, err = svc.Login(UserAdmin, "wrong-password"); err == nil {
		t.Fatal("bad password should fail")
	}

	roles, err := svc.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	var analystRoleID uint
	for _, role := range roles {
		if role.Code == RoleAnalyst {
			analystRoleID = role.ID
		}
	}
	created, err := svc.CreateUser(admin, UserInput{
		Username: "guest", Password: "guest123", DisplayName: "访客", RoleIDs: []uint{analystRoleID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Login("guest", "guest123"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateUser(analyst, UserPatch{ID: admin.UserID, Enabled: boolPtr(false)}); err == nil {
		t.Fatal("analyst must not disable admin")
	}
	if err = svc.DeleteUser(admin, admin.UserID); err == nil {
		t.Fatal("cannot delete self")
	}
	if _, err = svc.UpdateUser(admin, UserPatch{ID: admin.UserID, Enabled: boolPtr(false)}); err == nil {
		t.Fatal("cannot disable the only enabled admin")
	}

	second, err := svc.CreateUser(admin, UserInput{
		Username: "root2", Password: "root234", DisplayName: "第二管理员", RoleIDs: []uint{adminRoleID(t, roles)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateUser(admin, UserPatch{ID: second.ID, Enabled: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if err = svc.DeleteUser(admin, second.ID); err != nil {
		t.Fatal(err)
	}

	if _, err = svc.CreateRole(RoleInput{Code: "auditor", Name: "审计", Permissions: []string{"stock:quote:view"}}); err != nil {
		t.Fatal(err)
	}
	custom, err := svc.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	var auditorID uint
	for _, role := range custom {
		if role.Code == "auditor" {
			auditorID = role.ID
			if len(role.Permissions) != 1 {
				t.Fatalf("auditor perms %+v", role.Permissions)
			}
		}
		if role.Code == RoleAdmin && !role.BuiltIn {
			t.Fatal("admin should be built-in")
		}
	}
	if err = svc.DeleteRole(auditorID); err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		if role.BuiltIn {
			if err = svc.DeleteRole(role.ID); err == nil {
				t.Fatal("built-in role should stay")
			}
		}
	}

	if err = svc.ChangePassword(created.ID, "guest123", "guest456"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Login("guest", "guest456"); err != nil {
		t.Fatal(err)
	}
	if err = Seed(svc.db); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Login("guest", "guest456"); err != nil {
		t.Fatal("reseed must keep custom password")
	}
	if _, err = svc.Login(UserAdmin, PassAdmin); err != nil {
		t.Fatal("built-in admin password stays when the user already exists")
	}

	if err = svc.Logout(adminLogin.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Resolve("Bearer "+adminLogin.Token, ""); err == nil {
		t.Fatal("logged out token should fail")
	}
	static, err := svc.Resolve("Bearer super", "super")
	if err != nil || !static.Allows("stock:user:delete") {
		t.Fatal(err)
	}
}

func adminRoleID(t *testing.T, roles []RoleView) uint {
	t.Helper()
	for _, role := range roles {
		if role.Code == RoleAdmin {
			return role.ID
		}
	}
	t.Fatal("missing admin role")
	return 0
}

func boolPtr(v bool) *bool { return &v }
