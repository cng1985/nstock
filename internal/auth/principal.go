package auth

import "sort"

// SessionPermission 表示已登录即可调用，不写入角色权限表。
const SessionPermission = "auth:session"

type PermissionView struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type RoleBrief struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type Profile struct {
	ID          uint        `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"displayName"`
	Roles       []RoleBrief `json:"roles"`
	Permissions []string    `json:"permissions"`
}

type UserView struct {
	ID          uint        `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"displayName"`
	Enabled     bool        `json:"enabled"`
	Roles       []RoleBrief `json:"roles"`
	CreatedAt   string      `json:"createdAt"`
}

type RoleView struct {
	ID          uint     `json:"id"`
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BuiltIn     bool     `json:"builtIn"`
	Permissions []string `json:"permissions"`
}

type LoginResult struct {
	Token     string  `json:"token"`
	ExpiresAt string  `json:"expiresAt"`
	User      Profile `json:"user"`
}

type Principal struct {
	UserID      uint
	Username    string
	DisplayName string
	Roles       []RoleBrief
	Permissions []string
	permSet     map[string]struct{}
}

func (p Principal) Allows(code string) bool {
	if code == SessionPermission {
		return p.UserID != 0 || p.Username != ""
	}
	if p.permSet == nil {
		return false
	}
	_, ok := p.permSet[code]
	return ok
}

func (p Principal) HasRole(code string) bool {
	for _, role := range p.Roles {
		if role.Code == code {
			return true
		}
	}
	return false
}

func (p Principal) Profile() Profile {
	roles := p.Roles
	if roles == nil {
		roles = []RoleBrief{}
	}
	perms := append([]string(nil), p.Permissions...)
	sort.Strings(perms)
	if perms == nil {
		perms = []string{}
	}
	return Profile{
		ID:          p.UserID,
		Username:    p.Username,
		DisplayName: p.DisplayName,
		Roles:       roles,
		Permissions: perms,
	}
}

func PermissionViews() []PermissionView {
	out := make([]PermissionView, len(Catalog))
	copy(out, Catalog)
	return out
}

func staticPrincipal() Principal {
	codes := AllCodes()
	return Principal{
		Username:    "token",
		DisplayName: "访问令牌",
		Roles:       []RoleBrief{{Code: RoleAdmin, Name: "管理员"}},
		Permissions: codes,
		permSet:     setOf(codes),
	}
}

func setOf(codes []string) map[string]struct{} {
	out := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		out[code] = struct{}{}
	}
	return out
}
