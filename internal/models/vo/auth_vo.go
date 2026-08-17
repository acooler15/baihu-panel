package vo

// ===========================================================================
// Auth 相关视图对象（/api/v1/auth/* 与 /api/v1/auth/otp/*）
// ===========================================================================

// CurrentUserVO 当前用户信息
type CurrentUserVO struct {
	Username string `json:"username" description:"用户名"`
	Role     string `json:"role" description:"角色（admin/user）"`
}

// OTPStatusVO OTP 两步验证状态
type OTPStatusVO struct {
	OTPEnabled bool `json:"otp_enabled" description:"是否已开启两步验证"`
}

// OTPGenerateVO OTP 密钥生成结果
type OTPGenerateVO struct {
	Secret string `json:"secret" description:"TOTP 密钥（Base32）"`
	URL    string `json:"url" description:"otpauth:// 格式的二维码内容"`
}
