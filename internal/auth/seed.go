package auth

import (
	"errors"

	"nstock/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type builtInRole struct {
	Code        string
	Name        string
	Description string
}

type builtInUser struct {
	Username    string
	Password    string
	DisplayName string
	Role        string
}

func Seed(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := upsertPermissions(tx); err != nil {
			return err
		}
		roles := []builtInRole{
			{RoleAdmin, "管理员", "用户、角色和全部研究功能"},
			{RoleTrader, "交易员", "行情、预测、模拟交易和盯盘"},
			{RoleAnalyst, "研究员", "查看行情与研究记录，可以生成预测，不能下单"},
		}
		for _, role := range roles {
			if _, err := upsertBuiltInRole(tx, role); err != nil {
				return err
			}
		}
		users := []builtInUser{
			{UserAdmin, PassAdmin, "管理员", RoleAdmin},
			{UserTrader, PassTrader, "交易员", RoleTrader},
			{UserAnalyst, PassAnalyst, "研究员", RoleAnalyst},
		}
		for _, user := range users {
			if err := upsertBuiltInUser(tx, user); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertPermissions(tx *gorm.DB) error {
	for _, item := range Catalog {
		var row model.Permission
		err := tx.Where("code = ?", item.Code).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err = tx.Create(&model.Permission{Code: item.Code, Name: item.Name, GroupName: item.Group}).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if row.Name == item.Name && row.GroupName == item.Group {
			continue
		}
		row.Name = item.Name
		row.GroupName = item.Group
		if err = tx.Save(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func upsertBuiltInRole(tx *gorm.DB, spec builtInRole) (model.Role, error) {
	var role model.Role
	err := tx.Where("code = ?", spec.Code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = model.Role{Code: spec.Code, Name: spec.Name, Description: spec.Description, BuiltIn: true}
		if err = tx.Create(&role).Error; err != nil {
			return role, err
		}
		return role, replacePerms(tx, role.ID, presetCodes(spec.Code))
	}
	if err != nil {
		return role, err
	}
	if !role.BuiltIn {
		if err = tx.Model(&role).Update("built_in", true).Error; err != nil {
			return role, err
		}
		role.BuiltIn = true
	}
	var n int64
	if err = tx.Model(&model.RolePermission{}).Where("role_id = ?", role.ID).Count(&n).Error; err != nil {
		return role, err
	}
	if spec.Code == RoleAdmin || n == 0 {
		if err = replacePerms(tx, role.ID, presetCodes(spec.Code)); err != nil {
			return role, err
		}
	}
	return role, nil
}

func upsertBuiltInUser(tx *gorm.DB, spec builtInUser) error {
	var user model.User
	err := tx.Where("username = ?", spec.Username).First(&user).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(spec.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user = model.User{
		Username:     spec.Username,
		DisplayName:  spec.DisplayName,
		PasswordHash: string(hash),
		Enabled:      true,
	}
	if err = tx.Create(&user).Error; err != nil {
		return err
	}
	var role model.Role
	if err = tx.Where("code = ?", spec.Role).First(&role).Error; err != nil {
		return err
	}
	return tx.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID}).Error
}

func replacePerms(tx *gorm.DB, roleID uint, codes []string) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
		return err
	}
	if len(codes) == 0 {
		return nil
	}
	rows := make([]model.RolePermission, 0, len(codes))
	for _, code := range codes {
		rows = append(rows, model.RolePermission{RoleID: roleID, Code: code})
	}
	return tx.Create(&rows).Error
}
