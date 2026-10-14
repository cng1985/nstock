package auth

// Catalog 是接口使用的权限码。管理员角色始终拥有全部权限。
var Catalog = []PermissionView{
	{Code: "stock:system:view", Name: "查看系统状态", Group: "系统"},
	{Code: "stock:quote:view", Name: "查看行情", Group: "行情"},
	{Code: "stock:watch:view", Name: "查看自选", Group: "行情"},
	{Code: "stock:watch:create", Name: "添加自选", Group: "行情"},
	{Code: "stock:watch:delete", Name: "删除自选", Group: "行情"},
	{Code: "stock:predict:view", Name: "查看预测", Group: "预测"},
	{Code: "stock:predict:execute", Name: "生成预测", Group: "预测"},
	{Code: "stock:account:view", Name: "查看模拟账户", Group: "交易"},
	{Code: "stock:account:reset", Name: "重置模拟账户", Group: "交易"},
	{Code: "stock:order:view", Name: "查看委托", Group: "交易"},
	{Code: "stock:order:create", Name: "模拟下单", Group: "交易"},
	{Code: "stock:strategy:view", Name: "查看策略", Group: "量化"},
	{Code: "stock:strategy:create", Name: "创建策略", Group: "量化"},
	{Code: "stock:strategy:update", Name: "修改策略", Group: "量化"},
	{Code: "stock:strategy:delete", Name: "删除策略", Group: "量化"},
	{Code: "stock:strategy:backtest", Name: "回测策略", Group: "量化"},
	{Code: "stock:strategy:execute", Name: "执行策略", Group: "量化"},
	{Code: "stock:alert:view", Name: "查看盯盘", Group: "盯盘"},
	{Code: "stock:alert:create", Name: "添加盯盘", Group: "盯盘"},
	{Code: "stock:alert:update", Name: "修改盯盘", Group: "盯盘"},
	{Code: "stock:alert:delete", Name: "删除盯盘", Group: "盯盘"},
	{Code: "stock:monitor:view", Name: "查看扫描状态", Group: "盯盘"},
	{Code: "stock:monitor:execute", Name: "立即扫描", Group: "盯盘"},
	{Code: "stock:user:view", Name: "查看用户", Group: "权限"},
	{Code: "stock:user:create", Name: "创建用户", Group: "权限"},
	{Code: "stock:user:update", Name: "修改用户", Group: "权限"},
	{Code: "stock:user:delete", Name: "删除用户", Group: "权限"},
	{Code: "stock:role:view", Name: "查看角色", Group: "权限"},
	{Code: "stock:role:create", Name: "创建角色", Group: "权限"},
	{Code: "stock:role:update", Name: "修改角色", Group: "权限"},
	{Code: "stock:role:delete", Name: "删除角色", Group: "权限"},
}

const (
	RoleAdmin   = "admin"
	RoleTrader  = "trader"
	RoleAnalyst = "analyst"

	UserAdmin   = "admin"
	UserTrader  = "trader"
	UserAnalyst = "analyst"

	PassAdmin   = "admin123"
	PassTrader  = "trader123"
	PassAnalyst = "analyst123"
)

func AllCodes() []string {
	out := make([]string, len(Catalog))
	for i, item := range Catalog {
		out[i] = item.Code
	}
	return out
}

func knownPermission(code string) bool {
	for _, item := range Catalog {
		if item.Code == code {
			return true
		}
	}
	return false
}

func presetCodes(role string) []string {
	switch role {
	case RoleAdmin:
		return AllCodes()
	case RoleTrader:
		out := make([]string, 0, len(Catalog))
		for _, code := range AllCodes() {
			if code == "stock:account:reset" || hasPrefix(code, "stock:user:") || hasPrefix(code, "stock:role:") {
				continue
			}
			out = append(out, code)
		}
		return out
	case RoleAnalyst:
		return []string{
			"stock:system:view",
			"stock:quote:view",
			"stock:watch:view",
			"stock:predict:view",
			"stock:predict:execute",
			"stock:account:view",
			"stock:order:view",
			"stock:strategy:view",
			"stock:alert:view",
			"stock:monitor:view",
		}
	default:
		return nil
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
