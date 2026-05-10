package models


type CreateSysUser struct {
	Name     string   `json:"name" binding:"required"`    
	Phone    string   `json:"phone" binding:"required"`    
	Password string   `json:"password" binding:"required"` 
	Roles    []string `json:"roles" binding:"required"`    
}


type SysUser struct {
	ID        string   `json:"id"`      
	Name      string   `json:"name"`      
	Phone     string   `json:"phone"`    
	Roles     []string `json:"roles"`      
	CreatedAt string   `json:"created_at"` 
}
type SysUserLoginRequest struct {
	Phone    string `json:"phone" binding:"required"`
	Password string `json:"password" binding:"required"`
}
type SysUserLoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
