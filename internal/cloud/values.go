package cloud

import (
	"time"

	"github.com/cliffom/bambustand/internal/config"
)

// Cloud API paths, authentication identifiers, and local request limits.
const (
	emailCodeLogin     = "codeLogin"
	requestTimeout     = 15 * time.Second
	maxResponseBytes   = 2 * 1024 * 1024
	loginPath          = "/v1/user-service/user/login"
	sendCodePath       = "/v1/user-service/user/sendemail/code"
	csrfPath           = "/api/csrf"
	twoFactorPath      = "/api/sign-in/tfa"
	preferencesPath    = "/v1/design-user-service/my/preference"
	devicesPath        = "/v1/iot-service/api/user/bind"
	LoginVerifyCode    = "verifyCode"
	LoginTFA           = "tfa"
	csrfCookieName     = "bbl_csrf_token"
	tokenCookieName    = "token"
	csrfHeader         = "X-Bbl-Csrf-Token"
	mqttUsernamePrefix = "u_"
	jsonContentType    = "application/json"
	userAgent          = "bambu_network_agent/01.09.05.01"
	clientName         = "OrcaSlicer"
	clientType         = "slicer"
	clientVersion      = "01.09.05.51"
)

// endpoints keeps each region's API, web login, and broker addresses together.
type endpoints struct{ api, site, mqtt string }

func endpointsFor(region string) endpoints {
	if region == config.RegionChina {
		return endpoints{api: "https://api.bambulab.cn", site: "https://bambulab.cn", mqtt: "cn.mqtt.bambulab.com:8883"}
	}
	return endpoints{api: "https://api.bambulab.com", site: "https://bambulab.com", mqtt: "us.mqtt.bambulab.com:8883"}
}
