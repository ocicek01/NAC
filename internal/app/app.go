package app

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nac/internal/arpsnmpwalk"
	dhcpcollector "nac/internal/collector/dhcp"
	snmptrapcollector "nac/internal/collector/snmptrap"
	"nac/internal/config"
	"nac/internal/database"
	arpsnapshotdomain "nac/internal/domain/arpsnapshot"
	auditlogdomain "nac/internal/domain/auditlog"
	devicedomain "nac/internal/domain/device"
	dhcpdomain "nac/internal/domain/dhcpevent"
	discoveryjobdomain "nac/internal/domain/discoveryjob"
	enforcementdomain "nac/internal/domain/enforcement"
	guestidentitydomain "nac/internal/domain/guestidentity"
	macipbindingdomain "nac/internal/domain/macipbinding"
	policydomain "nac/internal/domain/policy"
	portendpointdomain "nac/internal/domain/portendpoint"
	porteventdomain "nac/internal/domain/portevent"
	radiuseventdomain "nac/internal/domain/radiusevent"
	sessiondomain "nac/internal/domain/session"
	snmptrapdomain "nac/internal/domain/snmptrap"
	switchdomain "nac/internal/domain/switchasset"
	topologydomain "nac/internal/domain/topology"
	trapwindowdomain "nac/internal/domain/trapwindow"
	"nac/internal/httpserver"
	"nac/internal/logging"
	arpsnapshotrepository "nac/internal/repository/arpsnapshot"
	auditlogrepository "nac/internal/repository/auditlog"
	devicerepository "nac/internal/repository/device"
	dhcprepository "nac/internal/repository/dhcpevent"
	discoveryjobrepository "nac/internal/repository/discoveryjob"
	enforcementrepository "nac/internal/repository/enforcement"
	guestidentityrepository "nac/internal/repository/guestidentity"
	macipbindingrepository "nac/internal/repository/macipbinding"
	macobservationrepository "nac/internal/repository/macobservation"
	policyrepository "nac/internal/repository/policy"
	portendpointrepository "nac/internal/repository/portendpoint"
	porteventrepository "nac/internal/repository/portevent"
	radiuseventrepository "nac/internal/repository/radiusevent"
	sessionrepository "nac/internal/repository/session"
	snmptraprepository "nac/internal/repository/snmptrap"
	switchrepository "nac/internal/repository/switchasset"
	switchportrepository "nac/internal/repository/switchport"
	topologyrepository "nac/internal/repository/topology"
	trapwindowrepository "nac/internal/repository/trapwindow"
	auditlogservice "nac/internal/service/auditlog"
	deviceservice "nac/internal/service/device"
	dhcpservice "nac/internal/service/dhcpevent"
	discoveryjobservice "nac/internal/service/discoveryjob"
	enforcementservice "nac/internal/service/enforcement"
	guestidentityservice "nac/internal/service/guestidentity"
	identitysourceservice "nac/internal/service/identitysource"
	maccorrelationservice "nac/internal/service/maccorrelation"
	maclookupservice "nac/internal/service/maclookup"
	macobservationservice "nac/internal/service/macobservation"
	policyservice "nac/internal/service/policy"
	portalservice "nac/internal/service/portal"
	portendpointservice "nac/internal/service/portendpoint"
	porteventservice "nac/internal/service/portevent"
	radiusauthservice "nac/internal/service/radiusauth"
	sessionservice "nac/internal/service/session"
	snmptrapservice "nac/internal/service/snmptrap"
	switchservice "nac/internal/service/switchasset"
	topologyservice "nac/internal/service/topology"
	trapwindowservice "nac/internal/service/trapwindow"
	"nac/internal/snmp"
)

type App struct {
	config     config.Config
	logger     *slog.Logger
	postgres   *pgxpool.Pool
	server     *httpserver.Server
	collector  *dhcpcollector.Collector
	traps      *snmptrapcollector.Collector
	trapWork   *trapwindowservice.Service
	deviceWork *deviceservice.Service
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger := logging.New(cfg.Log.Level)

	postgresPool, err := database.NewPostgresPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	var switchRepository switchdomain.Repository = switchrepository.NewPostgresRepository(postgresPool)
	var deviceRepository devicedomain.Repository = devicerepository.NewPostgresRepository(postgresPool)
	var auditLogRepository auditlogdomain.Repository = auditlogrepository.NewPostgresRepository(postgresPool)
	var guestIdentityRepository guestidentitydomain.Repository = guestidentityrepository.NewPostgresRepository(postgresPool)
	switchPortRepository := switchportrepository.NewPostgresRepository(postgresPool)
	var arpSnapshotRepository arpsnapshotdomain.Repository = arpsnapshotrepository.NewPostgresRepository(postgresPool)
	var macIPBindingRepository macipbindingdomain.Repository = macipbindingrepository.NewPostgresRepository(postgresPool)
	var portEndpointRepository portendpointdomain.Repository = portendpointrepository.NewPostgresRepository(postgresPool)
	var snmpTrapRepository snmptrapdomain.Repository = snmptraprepository.NewPostgresRepository(postgresPool)
	var trapWindowRepository trapwindowdomain.Repository = trapwindowrepository.NewPostgresRepository(postgresPool)
	snmpClient := snmp.NewClient()
	arpExternalCollector := arpsnmpwalk.New(cfg.SNMP)
	switchAssetService := switchservice.NewService(switchRepository, deviceRepository, switchPortRepository, cfg.SNMP, snmpClient)
	portEndpointService := portendpointservice.NewService(switchRepository, switchPortRepository, portEndpointRepository)
	macLookupService := maclookupservice.NewService(switchRepository, snmpClient)
	var topologyRepository topologydomain.Repository = topologyrepository.NewPostgresRepository(postgresPool)
	topologyService := topologyservice.NewService(topologyRepository, switchRepository, snmpClient)
	var discoveryJobRepository discoveryjobdomain.Repository = discoveryjobrepository.NewPostgresRepository(postgresPool)
	discoveryJobService := discoveryjobservice.NewService(discoveryJobRepository, switchRepository, switchPortRepository, arpSnapshotRepository, macIPBindingRepository, snmpClient, arpExternalCollector, topologyService)
	trapWindowService := trapwindowservice.NewService(logger, trapWindowRepository, discoveryJobRepository)
	macObservationRepository := macobservationrepository.NewPostgresRepository(postgresPool)
	macObservationService := macobservationservice.NewService(macObservationRepository)
	var policyRepository policydomain.Repository = policyrepository.NewPostgresRepository(postgresPool)
	var portEventRepository porteventdomain.Repository = porteventrepository.NewPostgresRepository(postgresPool)
	auditService := auditlogservice.NewService(auditLogRepository)
	policyEngineService := policyservice.NewService(policyRepository, auditService, policyservice.Config{EnforcementEnabled: cfg.Policy.EnforcementEnabled, DefaultDryRun: cfg.Policy.DefaultDryRun, ThresholdAllow: cfg.Policy.ThresholdAllow, ThresholdMonitor: cfg.Policy.ThresholdMonitor, ThresholdRestricted: cfg.Policy.ThresholdRestricted, ThresholdRegistration: cfg.Policy.ThresholdRegistration, TrustScore: policyservice.TrustScoreConfig{BaseScore: cfg.Policy.TrustBaseScore, LDAPRegistryMatch: cfg.Policy.TrustLDAPRegistryMatch, RegisteredOwner: cfg.Policy.TrustRegisteredOwner, KnownDeviceType: cfg.Policy.TrustKnownDeviceType, DepartmentPresent: cfg.Policy.TrustDepartmentPresent, DefaultVLANPresent: cfg.Policy.TrustDefaultVLANPresent, StableAttachment: cfg.Policy.TrustStableAttachment, LDAPNotFound: cfg.Policy.TrustLDAPNotFound, UnknownDeviceType: cfg.Policy.TrustUnknownDeviceType, RapidPortMovement: cfg.Policy.TrustRapidPortMovement, PreviousQuarantine: cfg.Policy.TrustPreviousQuarantine, IPMACAnomaly: cfg.Policy.TrustIPMACAnomaly, PortProfileMismatch: cfg.Policy.TrustPortProfileMismatch, RepeatedEnrichmentError: cfg.Policy.TrustRepeatedEnrichmentError}})
	var radiusEventRepository radiuseventdomain.Repository = radiuseventrepository.NewPostgresRepository(postgresPool)
	var dhcpEventRepository dhcpdomain.Repository = dhcprepository.NewPostgresRepository(postgresPool)
	var radiusSessionRepository sessiondomain.Repository = sessionrepository.NewPostgresRepository(postgresPool)
	radiusSessionService := sessionservice.NewService(radiusSessionRepository)
	var enforcementRepository enforcementdomain.Repository = enforcementrepository.NewPostgresRepository(postgresPool)
	enforcementEngineService := enforcementservice.NewService(enforcementRepository, switchRepository, radiusSessionRepository, cfg.Radius)
	guestIdentityService := guestidentityservice.NewService(guestIdentityRepository)
	timeout := time.Duration(cfg.Identity.HTTPTimeoutSeconds) * time.Second
	ldapResolver := identitysourceservice.Resolver(identitysourceservice.NewLDAPResolver(cfg.Identity))
	if ldapResolver == nil {
		ldapResolver = identitysourceservice.NewHTTPResolver("ldap", cfg.Identity.LDAPVerifyURL, timeout)
	}
	ldapDeviceResolver := identitysourceservice.NewLDAPDeviceResolver(cfg.Identity)
	staffResolver := identitysourceservice.NewHTTPResolver("staff_service", cfg.Identity.StaffVerifyURL, timeout)
	studentResolver := identitysourceservice.NewHTTPResolver("student_service", cfg.Identity.StudentVerifyURL, timeout)
	deviceService := deviceservice.NewService(
		logger,
		deviceRepository,
		policyEngineService,
		enforcementEngineService,
		switchPortRepository,
		portEndpointRepository,
		radiusSessionRepository,
		macIPBindingRepository,
		dhcpEventRepository,
		ldapDeviceResolver,
		auditService,
		parseVLANID(cfg.Radius.RegistrationVLAN),
		parseVLANID(cfg.Radius.GuestVLAN),
		parseVLANID(cfg.Radius.QuarantineVLAN),
		cfg.Feature.AutoEnforcementExecution,
		cfg.PostEnforcement.IPLearningEnabled,
		time.Duration(cfg.PostEnforcement.IPLearningWaitSec)*time.Second,
		time.Duration(cfg.PostEnforcement.IPRecheckSec)*time.Second,
		time.Duration(cfg.PostEnforcement.PortBounceDelaySec)*time.Second,
		cfg.PostEnforcement.PortBounceEnabled,
		cfg.PostEnforcement.MaxMACCountForBounce,
	)
	portalRegistrationService := portalservice.NewService(deviceService, ldapResolver, staffResolver, studentResolver, guestIdentityService, parseVLANID(cfg.Radius.GuestVLAN))
	radiusService := radiusauthservice.NewService(policyEngineService, cfg.Radius, radiusEventRepository, macObservationRepository, switchRepository, deviceService, radiusSessionRepository)
	macCorrelationService := maccorrelationservice.NewService(
		logger,
		macLookupService,
		topologyService,
		macObservationRepository,
		deviceService,
		switchRepository,
		snmpClient,
		cfg.Feature.Option82CorrelationEnabled,
	)
	dhcpEventService := dhcpservice.NewService(dhcpEventRepository, switchRepository, macCorrelationService.Handle)
	portEventService := porteventservice.NewService(portEventRepository, switchPortRepository, deviceService, auditService)
	trapForwarder := snmptrapservice.NewHTTPPortStatusForwarder(cfg.SNMPTrap.ForwardEnabled, cfg.SNMPTrap.ForwardURL, cfg.SNMPTrap.ForwardToken, time.Duration(cfg.SNMPTrap.ForwardTimeoutSec)*time.Second)
	snmpTrapService := snmptrapservice.NewService(logger, snmpTrapRepository, switchRepository, switchPortRepository, trapWindowService, trapForwarder)

	server := httpserver.New(cfg.App.Port, logger, dhcpEventService, switchAssetService, portEndpointService, macLookupService, macObservationService, topologyService, discoveryJobService, deviceService, guestIdentityService, portalRegistrationService, radiusSessionService, policyEngineService, enforcementEngineService, radiusService, portEventService, auditService)
	collector := dhcpcollector.New(cfg.DHCP, logger, dhcpEventService)
	trapCollector := snmptrapcollector.New(cfg.SNMPTrap, logger, snmpTrapService)

	return &App{
		config:     cfg,
		logger:     logger,
		postgres:   postgresPool,
		server:     server,
		collector:  collector,
		traps:      trapCollector,
		trapWork:   trapWindowService,
		deviceWork: deviceService,
	}, nil
}

func parseVLANID(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		if err := a.server.Start(); err != nil {
			errCh <- err
		}
	}()

	if a.config.DHCP.Enabled {
		go func() {
			if err := a.collector.Start(ctx); err != nil {
				errCh <- err
			}
		}()
	}
	if a.config.SNMPTrap.Enabled {
		go func() {
			if err := a.traps.Start(ctx); err != nil {
				errCh <- err
			}
		}()
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if a.trapWork == nil {
						continue
					}
					if err := a.trapWork.ProcessDue(ctx, 100); err != nil {
						a.logger.Warn("trap window processor failed", "error", err)
					}
				}
			}
		}()
	}

	if a.deviceWork != nil {
		go a.deviceWork.RunEnrichmentBackfill(ctx)
	}

	select {
	case <-ctx.Done():
		a.logger.Info("shutdown requested", "reason", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	a.postgres.Close()
	return nil
}
