package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/adnanh/webhook/internal/network"
)

func validateNetworkRoutes(routes []network.HookRoute) error {
	const reserved = "__webhook_network"
	hooksPath := strings.Trim(*hooksURLPrefix, "/")
	if hooksPath == "" || hooksPath == reserved || strings.HasPrefix(reserved, hooksPath+"/") || strings.HasPrefix(hooksPath, reserved+"/") {
		return errors.New("urlprefix overlaps the reserved network route")
	}
	statusPath := strings.Trim(*statusURLPrefix, "/")
	if statusPath == reserved || strings.HasPrefix(statusPath, reserved+"/") || strings.HasPrefix(reserved, statusPath+"/") {
		return errors.New("status-path overlaps the reserved network route")
	}
	if *adminEnabled {
		adminPath := strings.Trim(*adminURLPrefix, "/")
		if adminPath == reserved || strings.HasPrefix(adminPath, reserved+"/") || strings.HasPrefix(reserved, adminPath+"/") {
			return errors.New("admin-path overlaps the reserved network route")
		}
	}
	for _, route := range routes {
		if route.Path == network.ControlPath || route.Path == network.DataPath || route.Path == "/"+statusPath {
			return fmt.Errorf("hook route %q overlaps a reserved route", route.Path)
		}
		if *adminEnabled {
			adminPath := "/" + strings.Trim(*adminURLPrefix, "/")
			if route.Path == adminPath || strings.HasPrefix(route.Path, adminPath+"/") {
				return fmt.Errorf("hook route %q overlaps admin-path", route.Path)
			}
		}
	}
	return nil
}

func prepareWebhookAgent(cfg *network.AgentConfig) error {
	if cfg.WebhookOnlyViaHub {
		if socket != "" {
			return errors.New("webhook_only_via_hub cannot use a local socket")
		}
		return nil
	}
	for _, service := range cfg.Services {
		if service.Name == "webhook" {
			if service.RelayTo != "" {
				return errors.New("with_webhook cannot relay the webhook service to a downstream node")
			}
			return nil
		}
	}
	cfg.Services = append(cfg.Services, network.Service{Name: "webhook", InProcess: true})
	return nil
}
