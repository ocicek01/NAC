<?php

namespace App\Services;

use App\Models\SwitchPort;
use RuntimeException;
use Illuminate\Support\Facades\Validator;
use Illuminate\Validation\ValidationException;

class PortActionService
{
    public const ACTIONS = [
        'force_reauth',
        'coa_disconnect',
        'bounce_port',
        'disable_port',
        'enable_port',
        'move_vlan',
        'move_guest_vlan',
        'move_quarantine_vlan',
        'add_note',
    ];

    public function __construct(
        protected AuditLogService $auditLogService,
        protected NacApiClient $nacApiClient
    ) {
    }

    public function execute(SwitchPort $port, array $payload, array $auditContext = []): array
    {
        $validated = Validator::make($payload, [
            'action' => ['required', 'in:'.implode(',', self::ACTIONS)],
            'vlan_id' => ['nullable', 'integer', 'min:1', 'max:4094'],
            'note' => ['nullable', 'string', 'max:1000'],
        ])->validate();

        $this->guardProtectedPort($port, $validated['action']);

        $this->auditLogService->log($validated['action'], 'switch_port', $port->id, array_merge($auditContext, [
            'switch_id' => $port->switch_id,
            'switch_port_id' => $port->id,
            'old_value' => [
                'status' => $port->status,
                'nac_mode' => $port->nac_mode,
                'vlan_id' => $port->vlan_id,
            ],
            'new_value' => [
                'requested_action' => $validated['action'],
                'vlan_id' => $validated['vlan_id'] ?? null,
                'note' => $validated['note'] ?? null,
            ],
        ]));

        $execution = $this->executeSwitchAction($port, $validated);

        return [
            'status' => 'success',
            'action' => $validated['action'],
            'message' => $execution['message'],
            'execution' => $execution['execution'],
        ];
    }

    public function guardProtectedPort(SwitchPort $port, string $action): void
    {
        $dangerousActions = ['disable_port', 'move_vlan', 'move_guest_vlan', 'move_quarantine_vlan'];

        if (in_array($port->port_type, ['trunk', 'uplink'], true) && in_array($action, $dangerousActions, true)) {
            throw ValidationException::withMessages([
                'action' => 'Trunk/Uplink portlarda bu aksiyona izin verilmez.',
            ]);
        }
    }

    protected function executeSwitchAction(SwitchPort $port, array $validated): array
    {
        return match ($validated['action']) {
            'move_vlan', 'move_guest_vlan', 'move_quarantine_vlan' => $this->executeVLANMove($port, $validated),
            default => [
                'message' => 'Aksiyon audit log\'a yazildi. Gercek switch islemleri bu aksiyon icin henuz baglanmadi.',
                'execution' => [
                    'driver' => 'placeholder',
                    'queued' => false,
                ],
            ],
        };
    }

    protected function executeVLANMove(SwitchPort $port, array $validated): array
    {
        $port->loadMissing('switch');
        if (! $port->switch) {
            throw ValidationException::withMessages([
                'action' => 'Port switch kaydi bulunamadi.',
            ]);
        }

        $resolvedSwitch = $this->nacApiClient->resolveSwitch($port->switch->hostname, $port->switch->ip_address);
        if (! is_array($resolvedSwitch) || blank($resolvedSwitch['id'] ?? null)) {
            throw ValidationException::withMessages([
                'action' => 'Go switch kaydi bulunamadi.',
            ]);
        }

        $vlanID = $this->resolveTargetVLAN($validated);
        $method = $this->preferredExecutionMethod($resolvedSwitch);
        if ($method === null) {
            throw ValidationException::withMessages([
                'action' => 'Bu switch icin executable VLAN enforcement method bulunamadi.',
            ]);
        }

        $requestPayload = [
            'switch_id' => (string) ($resolvedSwitch['id'] ?? ''),
            'bridge_port' => 0,
            'if_index' => (int) ($port->if_index ?? 0),
            'interface_name' => (string) ($port->port_name ?? ''),
            'vlan_id' => $vlanID,
            'selected_method' => $method,
            'skip_port_bounce' => false,
        ];

        try {
            if ($method === 'snmp-write') {
                $result = $this->nacApiClient->executeSNMPPortVLAN($requestPayload);
                $driver = 'snmp-write';
            } else {
                $result = $this->nacApiClient->executeSSHPortVLAN($requestPayload);
                $driver = 'ssh';
            }
        } catch (RuntimeException $e) {
            throw ValidationException::withMessages([
                'action' => $e->getMessage(),
            ]);
        }

        $port->forceFill([
            'vlan_id' => $vlanID,
        ])->save();

        return [
            'message' => 'Port VLAN degisikligi uygulandi.',
            'execution' => [
                'driver' => $driver,
                'queued' => false,
                'vlan_id' => $vlanID,
                'result' => $result,
            ],
        ];
    }

    protected function preferredExecutionMethod(array $resolvedSwitch): ?string
    {
        $supportsSNMP = (bool) ($resolvedSwitch['supports_snmp_write'] ?? false);
        $supportsSSH = (bool) ($resolvedSwitch['supports_ssh_enforcement'] ?? false);

        if (! $supportsSNMP && ! $supportsSSH) {
            return null;
        }

        $vendor = strtolower(trim((string) ($resolvedSwitch['vendor'] ?? '')));
        $model = strtolower(trim((string) ($resolvedSwitch['model'] ?? '')));
        $name = strtolower(trim((string) ($resolvedSwitch['name'] ?? '')));

        $snmpPreferredVendors = ['hp', 'hpe', 'aruba', 'arubaos'];
        foreach ($snmpPreferredVendors as $needle) {
            if (str_contains($vendor, $needle) || str_contains($model, $needle) || str_contains($name, $needle)) {
                return $supportsSNMP ? 'snmp-write' : ($supportsSSH ? 'ssh' : null);
            }
        }

        if ($supportsSNMP) {
            return 'snmp-write';
        }

        if ($supportsSSH) {
            return 'ssh';
        }

        return null;
    }

    protected function resolveTargetVLAN(array $validated): int
    {
        return match ($validated['action']) {
            'move_guest_vlan' => (int) config('services.nac.guest_vlan', 300),
            'move_quarantine_vlan' => (int) config('services.nac.quarantine_vlan', 333),
            default => (int) ($validated['vlan_id'] ?? 0),
        };
    }
}
