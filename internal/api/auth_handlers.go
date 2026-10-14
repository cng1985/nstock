package api

import (
	"context"

	"nstock/internal/auth"
	"nstock/internal/ncode"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type userCreateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Enabled     *bool  `json:"enabled"`
	RoleIDs     []uint `json:"roleIds"`
}

type userUpdateRequest struct {
	ID          uint    `json:"id"`
	DisplayName *string `json:"displayName"`
	Password    *string `json:"password"`
	Enabled     *bool   `json:"enabled"`
	RoleIDs     *[]uint `json:"roleIds"`
}

type roleCreateRequest struct {
	Code        string   `json:"code"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type roleUpdateRequest struct {
	ID          uint      `json:"id"`
	Name        *string   `json:"name"`
	Description *string   `json:"description"`
	Permissions *[]string `json:"permissions"`
}

func (s *Server) login(c *gin.Context) {
	raw, err := readBody(c)
	if err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	if err = ncode.CheckTenant(raw); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	var req loginRequest
	if err = bind(raw, &req); err != nil {
		ncode.Write(c.Writer, err, nil)
		return
	}
	data, err := s.identity.Login(req.Username, req.Password)
	ncode.Write(c.Writer, err, data)
}

func (s *Server) logout(c *gin.Context) {
	s.handle(c, auth.SessionPermission, &emptyRequest{}, func(context.Context) (any, error) {
		token, err := auth.Bearer(c.GetHeader("Authorization"))
		if err != nil {
			return nil, err
		}
		if s.cfg.Token != "" && token == s.cfg.Token {
			return nil, nil
		}
		return nil, s.identity.Logout(token)
	})
}

func (s *Server) me(c *gin.Context) {
	s.handle(c, auth.SessionPermission, &emptyRequest{}, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return principal.Profile(), nil
	})
}

func (s *Server) password(c *gin.Context) {
	var req passwordRequest
	s.handle(c, auth.SessionPermission, &req, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return nil, s.identity.ChangePassword(principal.UserID, req.OldPassword, req.NewPassword)
	})
}

func (s *Server) userList(c *gin.Context) {
	s.handle(c, "stock:user:view", &emptyRequest{}, func(context.Context) (any, error) {
		return s.identity.ListUsers()
	})
}

func (s *Server) userCreate(c *gin.Context) {
	var req userCreateRequest
	s.handle(c, "stock:user:create", &req, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return s.identity.CreateUser(principal, auth.UserInput{
			Username:    req.Username,
			DisplayName: req.DisplayName,
			Password:    req.Password,
			Enabled:     req.Enabled,
			RoleIDs:     req.RoleIDs,
		})
	})
}

func (s *Server) userUpdate(c *gin.Context) {
	var req userUpdateRequest
	s.handle(c, "stock:user:update", &req, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return s.identity.UpdateUser(principal, auth.UserPatch{
			ID:          req.ID,
			DisplayName: req.DisplayName,
			Password:    req.Password,
			Enabled:     req.Enabled,
			RoleIDs:     req.RoleIDs,
		})
	})
}

func (s *Server) userDelete(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:user:delete", &req, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return nil, s.identity.DeleteUser(principal, req.ID)
	})
}

func (s *Server) roleList(c *gin.Context) {
	s.handleAny(c, []string{"stock:role:view", "stock:user:view", "stock:user:create", "stock:user:update"}, &emptyRequest{}, func(context.Context) (any, error) {
		return s.identity.ListRoles()
	})
}

func (s *Server) roleCreate(c *gin.Context) {
	var req roleCreateRequest
	s.handle(c, "stock:role:create", &req, func(context.Context) (any, error) {
		return s.identity.CreateRole(auth.RoleInput{
			Code:        req.Code,
			Name:        req.Name,
			Description: req.Description,
			Permissions: req.Permissions,
		})
	})
}

func (s *Server) roleUpdate(c *gin.Context) {
	var req roleUpdateRequest
	s.handle(c, "stock:role:update", &req, func(context.Context) (any, error) {
		principal, err := s.current(c)
		if err != nil {
			return nil, err
		}
		return s.identity.UpdateRole(principal, auth.RolePatch{
			ID:          req.ID,
			Name:        req.Name,
			Description: req.Description,
			Permissions: req.Permissions,
		})
	})
}

func (s *Server) roleDelete(c *gin.Context) {
	var req idRequest
	s.handle(c, "stock:role:delete", &req, func(context.Context) (any, error) {
		return nil, s.identity.DeleteRole(req.ID)
	})
}

func (s *Server) permissionList(c *gin.Context) {
	s.handle(c, "stock:role:view", &emptyRequest{}, func(context.Context) (any, error) {
		return auth.PermissionViews(), nil
	})
}
