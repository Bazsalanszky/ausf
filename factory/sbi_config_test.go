// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package factory

import (
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestSbiDualStackKeys(t *testing.T) {
	const content = `
configuration:
  sbi:
    scheme: http
    registerIPv4: POD_IP
    registerIPv6: POD_IPS
    bindingAddr: "::"
    bindingIPv4: 0.0.0.0
    preferredFamily: IPv6
    port: 29509
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	want := Sbi{
		Scheme:          "http",
		RegisterIPv4:    "POD_IP",
		RegisterIPv6:    "POD_IPS",
		BindingAddr:     "::",
		BindingIPv4:     "0.0.0.0",
		PreferredFamily: "IPv6",
		Port:            29509,
	}
	if got := *cfg.Configuration.Sbi; got != want {
		t.Errorf("Sbi = %+v, want %+v", got, want)
	}
}

// TestSbiLegacyConfigHasNoNewKeys checks that the shipped configuration,
// which predates the dual-stack keys, still parses with the new keys unset.
func TestSbiLegacyConfigHasNoNewKeys(t *testing.T) {
	origAusfConfig := AusfConfig
	defer func() { AusfConfig = origAusfConfig }()
	if err := InitConfigFactory("ausfcfg.yaml"); err != nil {
		t.Fatalf("InitConfigFactory: %v", err)
	}
	sbi := AusfConfig.Configuration.Sbi
	if sbi.RegisterIPv4 != "1.1.1.1" || sbi.BindingIPv4 != "0.0.0.0" {
		t.Errorf("legacy keys = %q, %q", sbi.RegisterIPv4, sbi.BindingIPv4)
	}
	if sbi.RegisterIPv6 != "" || sbi.BindingAddr != "" || sbi.PreferredFamily != "" {
		t.Errorf("new keys should be unset, got %+v", *sbi)
	}
}
