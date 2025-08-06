package monitoring

import (
	"io"
	"net/http"
	"regexp"
)

var userAgent = "curl/8.0.1"

func GetIPv4Address() (string, error) {
	webAPIs := []string{
		"https://www.visa.cn/cdn-cgi/trace",
		"https://www.qualcomm.cn/cdn-cgi/trace",
		"https://www.toutiao.com/stream/widget/local_weather/data/",
		"https://edge-ip.html.zone/geo",
		"https://vercel-ip.html.zone/geo",
		"http://ipv4.ip.sb",
		"https://api.ipify.org?format=json"}

	for _, api := range webAPIs {
		// get ipv4
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		// 使用正则表达式从响应体中提取IPv4地址
		re := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
		ipv4 := re.FindString(string(body))
		if ipv4 != "" {
			return ipv4, nil
		}
	}
	return "", nil
}

func GetIPv6Address() (string, error) {
	webAPIs := []string{
		"https://v6.ip.zxinc.org/info.php?type=json",
		"https://api6.ipify.org?format=json",
		"https://ipv6.icanhazip.com",
		"https://api-ipv6.ip.sb/geoip"}

	for _, api := range webAPIs {
		// get ipv6
		req, err := http.NewRequest("GET", api, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		// 使用正则表达式从响应体中提取IPv6地址
		re := regexp.MustCompile(`(([0-9A-Fa-f]{1,4}:){7})([0-9A-Fa-f]{1,4})|(([0-9A-Fa-f]{1,4}:){1,6}:)(([0-9A-Fa-f]{1,4}:){0,4})([0-9A-Fa-f]{1,4})`)
		ipv6 := re.FindString(string(body))
		if ipv6 != "" {
			return ipv6, nil
		}
	}
	return "", nil
}

func GetIPAddress() (ipv4, ipv6 string, err error) {
	ipv4, err = GetIPv4Address()
	if err != nil {
		ipv4 = ""
	}
	ipv6, err = GetIPv6Address()
	if err != nil {
		ipv6 = ""
	}

	return ipv4, ipv6, nil
}
