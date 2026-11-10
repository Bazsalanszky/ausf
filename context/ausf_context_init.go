// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package context

import (
	"cmp"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omec-project/ausf/factory"
	"github.com/omec-project/ausf/logger"
	"github.com/omec-project/openapi/v2/models"
	openapiutils "github.com/omec-project/openapi/v2/utils"
	"github.com/omec-project/util/sbiaddr"
)

func InitAusfContext(context *AUSFContext) {
	config := factory.AusfConfig
	logger.InitLog.Infof("ausfconfig Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	configuration := config.Configuration
	sbi := configuration.Sbi

	context.NfId = uuid.New().String()
	context.GroupID = configuration.GroupId
	context.NrfUri = configuration.NrfUri
	context.UriScheme = models.UriScheme(configuration.Sbi.Scheme) // default uri scheme
	context.RegisterIPv4 = factory.AUSF_DEFAULT_IPV4               // default localhost
	context.RegisterIPv6 = ""
	context.SbiPreference = sbiaddr.PreferIPv4
	context.SBIPort = factory.AUSF_DEFAULT_PORT_INT // default port
	if sbi != nil {
		configureSbiSettings(context, sbi)
		configureBindingIPv4(context, sbi)
	}

	context.Url = context.GetSbiUri()
	context.EnableNrfCaching = configuration.EnableNrfCaching
	if configuration.EnableNrfCaching {
		if configuration.NrfCacheEvictionInterval == 0 {
			context.NrfCacheEvictionInterval = time.Duration(900) // 15 mins
		} else {
			context.NrfCacheEvictionInterval = time.Duration(configuration.NrfCacheEvictionInterval)
		}
	}

	// context.NfService
	context.NfService = make(map[models.ServiceName]models.NFService)
	AddNfServices(&context.NfService, &config, context)
	logger.ContextLog.Infoln("ausf context:", context)
}

func configureSbiSettings(context *AUSFContext, sbi *factory.Sbi) {
	configureRegisterAddrs(context, sbi)
	if sbi.Port != 0 {
		context.SBIPort = sbi.Port
	}

	switch sbi.Scheme {
	case "https":
		context.UriScheme = models.URISCHEME_HTTPS
	case "", "http":
		context.UriScheme = models.URISCHEME_HTTP
	default:
		logger.InitLog.Warnf("unsupported SBI scheme %q; defaulting to http", sbi.Scheme)
		context.UriScheme = models.URISCHEME_HTTP
	}
	if tls := sbi.TLS; tls != nil {
		if tls.Key != "" {
			context.Key = tls.Key
		}
		if tls.PEM != "" {
			context.PEM = tls.PEM
		}
	}
}

// configureRegisterAddrs sets RegisterIPv4 and RegisterIPv6 from the
// registerIPv4 and registerIPv6 settings. Each may be an address of either
// family, a host name, or the name of an environment variable such as POD_IP
// or POD_IPS; every value goes to the field of its own kind. When nothing
// usable is configured, the AUSF keeps its IPv4 default.
func configureRegisterAddrs(context *AUSFContext, sbi *factory.Sbi) {
	addrs, err := sbiaddr.ResolveRegister(sbi.RegisterIPv4, sbi.RegisterIPv6)
	if err != nil {
		logger.InitLog.Warnf("SBI register address: %v", err)
	}
	if addrs.Empty() {
		addrs.IPv4 = factory.AUSF_DEFAULT_IPV4
	}
	if addrs.FQDN != "" && addrs.IPv4 != "" {
		logger.InitLog.Warnf("SBI register address: registering FQDN %s, not IPv4 %s", addrs.FQDN, addrs.IPv4)
	}
	context.RegisterIPv4 = cmp.Or(addrs.FQDN, addrs.IPv4)
	context.RegisterIPv6 = addrs.IPv6

	if context.SbiPreference, err = sbiaddr.ParsePreference(sbi.PreferredFamily); err != nil {
		logger.InitLog.Warnf("SBI preferredFamily: %v; using IPv4", err)
	}
}

// configureBindingIPv4 sets the SBI listen host from bindingAddr, or from its
// legacy alias bindingIPv4. Either may name an environment variable.
func configureBindingIPv4(context *AUSFContext, sbi *factory.Sbi) {
	configured := cmp.Or(strings.TrimSpace(sbi.BindingAddr), strings.TrimSpace(sbi.BindingIPv4))
	switch {
	case configured == "":
		logger.InitLog.Warnln("error parsing ServerIPv4 address as string. Using the 0.0.0.0 address as default")
	case os.Getenv(configured) != "":
		logger.InitLog.Infoln("parsing ServerIPv4 address from ENV Variable")
	}
	var err error
	if context.BindingIPv4, err = sbiaddr.BindHost(sbi.BindingAddr, sbi.BindingIPv4); err != nil {
		logger.InitLog.Warnf("SBI binding address: %v", err)
	}
}

func AddNfServices(serviceMap *map[models.ServiceName]models.NFService, config *factory.Config, context *AUSFContext) {
	var nfService models.NFService
	var nfServiceVersions []models.NFServiceVersion
	services := *serviceMap

	// nausf-auth
	nfService.SetServiceInstanceId(context.NfId)
	nfService.SetServiceName(models.SERVICENAME_NAUSF_AUTH)
	addrs := context.SbiAddrs()
	ipEndPoints := openapiutils.NewIpEndPoints(addrs.IPv4, addrs.IPv6, int32(context.SBIPort))
	if addrs.FQDN != "" {
		nfService.SetFqdn(addrs.FQDN)
	}
	nfService.SetApiPrefix(context.GetSbiUri())

	var nfServiceVersion models.NFServiceVersion
	nfServiceVersion.SetApiFullVersion(config.Info.Version)
	nfServiceVersion.SetApiVersionInUri("v1")
	nfServiceVersions = append(nfServiceVersions, nfServiceVersion)

	nfService.SetScheme(context.UriScheme)
	nfService.SetNfServiceStatus(models.NFSERVICESTATUS_REGISTERED)

	nfService.SetIpEndPoints(ipEndPoints)
	nfService.SetVersions(nfServiceVersions)
	services[models.SERVICENAME_NAUSF_AUTH] = nfService
}
