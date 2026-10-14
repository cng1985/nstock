package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"nstock/internal/model"
	"nstock/internal/ncode"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const sessionTTL = 7 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

type UserInput struct {
	Username    string
	DisplayName string
	Password    string
	Enabled     *bool
	RoleIDs     []uint
}

type UserPatch struct {
	ID          uint
	DisplayName *string
	Password    *string
	Enabled     *bool
	RoleIDs     *[]uint
}

type RoleInput struct {
	Code        string
	Name        string
	Description string
	Permissions []string
}

type RolePatch struct {
	ID          uint
	Name        *string
	Description *string
	Permissions *[]string
}

func (s *Service) Login(username, password string) (LoginResult, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" || password == "" {
		return LoginResult{}, ncode.Fail(ncode.CodeBadRequest, "请输入用户名和密码")
	}
	var user model.User
	err := s.db.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil) {
		return LoginResult{}, ncode.Fail(ncode.CodeUnauthorized, "用户名或密码错误")
	}
	if err != nil {
		return LoginResult{}, err
	}
	if !user.Enabled {
		return LoginResult{}, ncode.Fail(ncode.CodeUnauthorized, "账号已停用")
	}
	_ = s.db.Where("expires_at < ?", time.Now()).Delete(&model.Session{}).Error
	token, hash, err := newToken()
	if err != nil {
		return LoginResult{}, ncode.Fail(ncode.CodeError, "登录失败")
	}
	sess := model.Session{TokenHash: hash, UserID: user.ID, ExpiresAt: time.Now().Add(sessionTTL)}
	if err = s.db.Create(&sess).Error; err != nil {
		return LoginResult{}, err
	}
	profile, err := s.profileOf(user)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, ExpiresAt: ncode.FormatTime(sess.ExpiresAt), User: profile}, nil
}

func (s *Service) Logout(token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	return s.db.Where("token_hash = ?", hashToken(token)).Delete(&model.Session{}).Error
}

func (s *Service) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if userID == 0 {
		return ncode.Fail(ncode.CodeBadRequest, "当前身份不能修改密码")
	}
	if oldPassword == "" || newPassword == "" {
		return ncode.Fail(ncode.CodeBadRequest, "请填写原密码和新密码")
	}
	if err := checkPassword(newPassword); err != nil {
		return err
	}
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return ncode.Fail(ncode.CodeUnauthorized, "未认证")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return ncode.Fail(ncode.CodeBadRequest, "原密码不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return ncode.Fail(ncode.CodeError, "修改密码失败")
	}
	return s.db.Model(&user).Updates(map[string]any{
		"password_hash": string(hash),
		"updated_at":    time.Now(),
	}).Error
}

func Bearer(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", nil
	}
	if !strings.HasPrefix(header, "Bearer ") {
		return "", ncode.Fail(ncode.CodeUnauthorized, "认证方式应为 Bearer token")
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), nil
}

func (s *Service) Resolve(header, staticToken string) (Principal, error) {
	token, err := Bearer(header)
	if err != nil {
		return Principal{}, err
	}
	if staticToken != "" && token != "" && tokenEqual(token, staticToken) {
		return staticPrincipal(), nil
	}
	if token == "" {
		return Principal{}, ncode.Fail(ncode.CodeUnauthorized, "未认证")
	}
	return s.principalFromToken(token)
}

func (s *Service) ListUsers() ([]UserView, error) {
	var users []model.User
	if err := s.db.Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	out := make([]UserView, 0, len(users))
	for _, user := range users {
		view, err := s.userView(user)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) CreateUser(actor Principal, in UserInput) (UserView, error) {
	username, err := normalizeUsername(in.Username)
	if err != nil {
		return UserView{}, err
	}
	display := strings.TrimSpace(in.DisplayName)
	if display == "" {
		display = username
	}
	display, err = checkName(display, "显示名")
	if err != nil {
		return UserView{}, err
	}
	if err = checkPassword(in.Password); err != nil {
		return UserView{}, err
	}
	roleIDs := uniqueUints(in.RoleIDs)
	if err = s.ensureRoles(roleIDs); err != nil {
		return UserView{}, err
	}
	grantsAdmin, err := s.roleIDsIncludeAdmin(roleIDs)
	if err != nil {
		return UserView{}, err
	}
	if grantsAdmin && !actor.HasRole(RoleAdmin) {
		return UserView{}, ncode.Fail(ncode.CodeForbidden, "只有管理员可以分配管理员角色")
	}
	var n int64
	if err = s.db.Model(&model.User{}).Where("username = ?", username).Count(&n).Error; err != nil {
		return UserView{}, err
	}
	if n > 0 {
		return UserView{}, ncode.Fail(ncode.CodeConflict, "用户名已存在")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return UserView{}, ncode.Fail(ncode.CodeError, "创建用户失败")
	}
	user := model.User{Username: username, DisplayName: display, PasswordHash: string(hash), Enabled: enabled}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return setUserRoles(tx, user.ID, roleIDs)
	})
	if err != nil {
		return UserView{}, err
	}
	return s.userView(user)
}

func (s *Service) UpdateUser(actor Principal, in UserPatch) (UserView, error) {
	if in.ID == 0 {
		return UserView{}, ncode.Fail(ncode.CodeBadRequest, "请提供用户编号")
	}
	var user model.User
	if err := s.db.First(&user, in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserView{}, ncode.Fail(ncode.CodeNotFound, "用户不存在")
		}
		return UserView{}, err
	}
	targetAdmin, err := s.hasRole(user.ID, RoleAdmin)
	if err != nil {
		return UserView{}, err
	}
	if targetAdmin && !actor.HasRole(RoleAdmin) {
		return UserView{}, ncode.Fail(ncode.CodeForbidden, "只有管理员可以修改管理员")
	}
	if in.Enabled != nil && !*in.Enabled && actor.UserID == user.ID {
		return UserView{}, ncode.Fail(ncode.CodeBadRequest, "不能停用当前登录用户")
	}
	willEnable := user.Enabled
	if in.Enabled != nil {
		willEnable = *in.Enabled
	}
	willAdmin := targetAdmin
	var roleIDs []uint
	if in.RoleIDs != nil {
		roleIDs = uniqueUints(*in.RoleIDs)
		if err = s.ensureRoles(roleIDs); err != nil {
			return UserView{}, err
		}
		willAdmin, err = s.roleIDsIncludeAdmin(roleIDs)
		if err != nil {
			return UserView{}, err
		}
		if willAdmin && !actor.HasRole(RoleAdmin) {
			return UserView{}, ncode.Fail(ncode.CodeForbidden, "只有管理员可以分配管理员角色")
		}
	}
	enabledAdmin, err := s.isEnabledAdmin(user.ID)
	if err != nil {
		return UserView{}, err
	}
	if enabledAdmin && !(willEnable && willAdmin) {
		count, err := s.countEnabledAdmins()
		if err != nil {
			return UserView{}, err
		}
		if count <= 1 {
			return UserView{}, ncode.Fail(ncode.CodeBadRequest, "至少保留一名启用的管理员")
		}
	}
	updates := map[string]any{"updated_at": time.Now()}
	if in.DisplayName != nil {
		name, err := checkName(*in.DisplayName, "显示名")
		if err != nil {
			return UserView{}, err
		}
		updates["display_name"] = name
		user.DisplayName = name
	}
	if in.Enabled != nil {
		updates["enabled"] = *in.Enabled
		user.Enabled = *in.Enabled
	}
	if in.Password != nil {
		if err = checkPassword(*in.Password); err != nil {
			return UserView{}, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*in.Password), bcrypt.DefaultCost)
		if err != nil {
			return UserView{}, ncode.Fail(ncode.CodeError, "修改用户失败")
		}
		updates["password_hash"] = string(hash)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			return err
		}
		if in.RoleIDs != nil {
			return setUserRoles(tx, user.ID, roleIDs)
		}
		return nil
	})
	if err != nil {
		return UserView{}, err
	}
	if !user.Enabled {
		_ = s.db.Where("user_id = ?", user.ID).Delete(&model.Session{}).Error
	}
	return s.userView(user)
}

func (s *Service) DeleteUser(actor Principal, id uint) error {
	if id == 0 {
		return ncode.Fail(ncode.CodeBadRequest, "请提供用户编号")
	}
	if actor.UserID == id {
		return ncode.Fail(ncode.CodeBadRequest, "不能删除当前登录用户")
	}
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ncode.Fail(ncode.CodeNotFound, "用户不存在")
		}
		return err
	}
	targetAdmin, err := s.hasRole(user.ID, RoleAdmin)
	if err != nil {
		return err
	}
	if targetAdmin && !actor.HasRole(RoleAdmin) {
		return ncode.Fail(ncode.CodeForbidden, "只有管理员可以删除管理员")
	}
	enabledAdmin, err := s.isEnabledAdmin(user.ID)
	if err != nil {
		return err
	}
	if enabledAdmin {
		count, err := s.countEnabledAdmins()
		if err != nil {
			return err
		}
		if count <= 1 {
			return ncode.Fail(ncode.CodeBadRequest, "至少保留一名启用的管理员")
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, user.ID).Error
	})
}

func (s *Service) ListRoles() ([]RoleView, error) {
	var roles []model.Role
	if err := s.db.Order("id asc").Find(&roles).Error; err != nil {
		return nil, err
	}
	out := make([]RoleView, 0, len(roles))
	for _, role := range roles {
		view, err := s.roleView(role)
		if err != nil {
			return nil, err
		}
		out = append(out, view)
	}
	return out, nil
}

func (s *Service) CreateRole(in RoleInput) (RoleView, error) {
	code, err := normalizeUsername(in.Code)
	if err != nil {
		return RoleView{}, ncode.Fail(ncode.CodeBadRequest, "角色标识需以字母开头，3 到 32 位，只能包含字母、数字和下划线")
	}
	if code == RoleAdmin {
		return RoleView{}, ncode.Fail(ncode.CodeBadRequest, "不能创建管理员角色")
	}
	name, err := checkName(in.Name, "角色名称")
	if err != nil {
		return RoleView{}, err
	}
	desc, err := checkDesc(in.Description)
	if err != nil {
		return RoleView{}, err
	}
	perms, err := normalizePerms(in.Permissions)
	if err != nil {
		return RoleView{}, err
	}
	var n int64
	if err = s.db.Model(&model.Role{}).Where("code = ?", code).Count(&n).Error; err != nil {
		return RoleView{}, err
	}
	if n > 0 {
		return RoleView{}, ncode.Fail(ncode.CodeConflict, "角色标识已存在")
	}
	role := model.Role{Code: code, Name: name, Description: desc, BuiltIn: false}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replacePerms(tx, role.ID, perms)
	})
	if err != nil {
		return RoleView{}, err
	}
	return s.roleView(role)
}

func (s *Service) UpdateRole(actor Principal, in RolePatch) (RoleView, error) {
	if in.ID == 0 {
		return RoleView{}, ncode.Fail(ncode.CodeBadRequest, "请提供角色编号")
	}
	var role model.Role
	if err := s.db.First(&role, in.ID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RoleView{}, ncode.Fail(ncode.CodeNotFound, "角色不存在")
		}
		return RoleView{}, err
	}
	if role.Code == RoleAdmin && !actor.HasRole(RoleAdmin) {
		return RoleView{}, ncode.Fail(ncode.CodeForbidden, "只有管理员可以修改管理员角色")
	}
	updates := map[string]any{"updated_at": time.Now()}
	if in.Name != nil {
		name, err := checkName(*in.Name, "角色名称")
		if err != nil {
			return RoleView{}, err
		}
		updates["name"] = name
		role.Name = name
	}
	if in.Description != nil {
		desc, err := checkDesc(*in.Description)
		if err != nil {
			return RoleView{}, err
		}
		updates["description"] = desc
		role.Description = desc
	}
	var perms []string
	if role.Code == RoleAdmin {
		perms = AllCodes()
		if in.Permissions != nil {
			got, err := normalizePerms(*in.Permissions)
			if err != nil {
				return RoleView{}, err
			}
			if !sameCodes(got, perms) {
				return RoleView{}, ncode.Fail(ncode.CodeBadRequest, "管理员角色必须保留全部权限")
			}
		}
	} else if in.Permissions != nil {
		var err error
		perms, err = normalizePerms(*in.Permissions)
		if err != nil {
			return RoleView{}, err
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Role{}).Where("id = ?", role.ID).Updates(updates).Error; err != nil {
			return err
		}
		if role.Code == RoleAdmin || in.Permissions != nil {
			return replacePerms(tx, role.ID, perms)
		}
		return nil
	})
	if err != nil {
		return RoleView{}, err
	}
	return s.roleView(role)
}

func (s *Service) DeleteRole(id uint) error {
	if id == 0 {
		return ncode.Fail(ncode.CodeBadRequest, "请提供角色编号")
	}
	var role model.Role
	if err := s.db.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ncode.Fail(ncode.CodeNotFound, "角色不存在")
		}
		return err
	}
	if role.BuiltIn || role.Code == RoleAdmin || role.Code == RoleTrader || role.Code == RoleAnalyst {
		return ncode.Fail(ncode.CodeBadRequest, "内置角色不能删除")
	}
	var n int64
	if err := s.db.Model(&model.UserRole{}).Where("role_id = ?", role.ID).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return ncode.Fail(ncode.CodeBadRequest, "角色仍分配给用户，请先解除")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", role.ID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Role{}, role.ID).Error
	})
}

func (s *Service) principalFromToken(token string) (Principal, error) {
	var sess model.Session
	err := s.db.Where("token_hash = ?", hashToken(token)).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Principal{}, ncode.Fail(ncode.CodeUnauthorized, "未认证")
	}
	if err != nil {
		return Principal{}, err
	}
	if !sess.ExpiresAt.After(time.Now()) {
		_ = s.db.Delete(&sess).Error
		return Principal{}, ncode.Fail(ncode.CodeUnauthorized, "登录已过期")
	}
	var user model.User
	if err = s.db.First(&user, sess.UserID).Error; err != nil {
		return Principal{}, ncode.Fail(ncode.CodeUnauthorized, "未认证")
	}
	if !user.Enabled {
		_ = s.db.Where("user_id = ?", user.ID).Delete(&model.Session{}).Error
		return Principal{}, ncode.Fail(ncode.CodeUnauthorized, "账号已停用")
	}
	return s.principalOf(user)
}

func (s *Service) profileOf(user model.User) (Profile, error) {
	p, err := s.principalOf(user)
	if err != nil {
		return Profile{}, err
	}
	return p.Profile(), nil
}

func (s *Service) principalOf(user model.User) (Principal, error) {
	roles, err := s.rolesOf(user.ID)
	if err != nil {
		return Principal{}, err
	}
	codes, err := s.permissionCodes(user.ID)
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Roles:       roles,
		Permissions: codes,
		permSet:     setOf(codes),
	}, nil
}

func (s *Service) userView(user model.User) (UserView, error) {
	roles, err := s.rolesOf(user.ID)
	if err != nil {
		return UserView{}, err
	}
	if roles == nil {
		roles = []RoleBrief{}
	}
	return UserView{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Enabled:     user.Enabled,
		Roles:       roles,
		CreatedAt:   ncode.FormatTime(user.CreatedAt),
	}, nil
}

func (s *Service) roleView(role model.Role) (RoleView, error) {
	var codes []string
	err := s.db.Model(&model.RolePermission{}).Where("role_id = ?", role.ID).Order("code asc").Pluck("code", &codes).Error
	if err != nil {
		return RoleView{}, err
	}
	if codes == nil {
		codes = []string{}
	}
	return RoleView{
		ID:          role.ID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		BuiltIn:     role.BuiltIn,
		Permissions: codes,
	}, nil
}

func (s *Service) rolesOf(userID uint) ([]RoleBrief, error) {
	var roles []model.Role
	err := s.db.Raw(`
		SELECT roles.id, roles.code, roles.name
		FROM roles
		JOIN user_roles ON user_roles.role_id = roles.id
		WHERE user_roles.user_id = ?
		ORDER BY roles.id
	`, userID).Scan(&roles).Error
	if err != nil {
		return nil, err
	}
	out := make([]RoleBrief, 0, len(roles))
	for _, role := range roles {
		out = append(out, RoleBrief{ID: role.ID, Code: role.Code, Name: role.Name})
	}
	return out, nil
}

func (s *Service) permissionCodes(userID uint) ([]string, error) {
	var codes []string
	err := s.db.Raw(`
		SELECT DISTINCT role_permissions.code
		FROM user_roles
		JOIN role_permissions ON role_permissions.role_id = user_roles.role_id
		WHERE user_roles.user_id = ?
		ORDER BY role_permissions.code
	`, userID).Scan(&codes).Error
	return codes, err
}

func (s *Service) ensureRoles(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	var n int64
	if err := s.db.Model(&model.Role{}).Where("id IN ?", ids).Count(&n).Error; err != nil {
		return err
	}
	if int(n) != len(ids) {
		return ncode.Fail(ncode.CodeBadRequest, "角色不存在")
	}
	return nil
}

func (s *Service) roleIDsIncludeAdmin(ids []uint) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}
	var n int64
	err := s.db.Model(&model.Role{}).Where("id IN ? AND code = ?", ids, RoleAdmin).Count(&n).Error
	return n > 0, err
}

func (s *Service) hasRole(userID uint, code string) (bool, error) {
	var n int64
	err := s.db.Raw(`
		SELECT COUNT(*)
		FROM user_roles
		JOIN roles ON roles.id = user_roles.role_id
		WHERE user_roles.user_id = ? AND roles.code = ?
	`, userID, code).Scan(&n).Error
	return n > 0, err
}

func (s *Service) isEnabledAdmin(userID uint) (bool, error) {
	var user model.User
	if err := s.db.Select("id", "enabled").First(&user, userID).Error; err != nil {
		return false, err
	}
	if !user.Enabled {
		return false, nil
	}
	return s.hasRole(userID, RoleAdmin)
}

func (s *Service) countEnabledAdmins() (int64, error) {
	var n int64
	err := s.db.Raw(`
		SELECT COUNT(DISTINCT users.id)
		FROM users
		JOIN user_roles ON user_roles.user_id = users.id
		JOIN roles ON roles.id = user_roles.role_id
		WHERE roles.code = ? AND users.enabled = ?
	`, RoleAdmin, true).Scan(&n).Error
	return n, err
}

func setUserRoles(tx *gorm.DB, userID uint, roleIDs []uint) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, 0, len(roleIDs))
	for _, id := range roleIDs {
		rows = append(rows, model.UserRole{UserID: userID, RoleID: id})
	}
	return tx.Create(&rows).Error
}

func normalizeUsername(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if !usernamePattern.MatchString(raw) {
		return "", ncode.Fail(ncode.CodeBadRequest, "用户名需以字母开头，3 到 32 位，只能包含字母、数字和下划线")
	}
	return raw, nil
}

func checkName(raw, label string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len([]rune(raw)) > 32 {
		return "", ncode.Fail(ncode.CodeBadRequest, label+"长度应为 1 到 32 个字符")
	}
	return raw, nil
}

func checkDesc(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len([]rune(raw)) > 80 {
		return "", ncode.Fail(ncode.CodeBadRequest, "角色说明不能超过 80 个字符")
	}
	return raw, nil
}

func checkPassword(password string) error {
	if len([]rune(password)) < 6 || len(password) > 72 {
		return ncode.Fail(ncode.CodeBadRequest, "密码长度应为 6 到 72 位")
	}
	return nil
}

func normalizePerms(codes []string) ([]string, error) {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if !knownPermission(code) {
			return nil, ncode.Fail(ncode.CodeBadRequest, "未知权限: "+code)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	sort.Strings(out)
	return out, nil
}

func sameCodes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func uniqueUints(ids []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func newToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func tokenEqual(a, b string) bool {
	if a == "" || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
