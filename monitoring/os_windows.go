//go:build windows
// +build windows

package monitoring

import (
	"golang.org/x/sys/windows/registry"
)

func OSName() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Microsoft Windows"
	}
	defer key.Close()

	productName, _, err := key.GetStringValue("ProductName")
	if err != nil {
		return "Microsoft Windows"
	}

	return productName
}
