<?php

namespace App\Services;

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Log;
use RuntimeException;
use Throwable;

class NacApiClient
{
    protected function client(): PendingRequest
    {
        return $this->baseClient()
            ->acceptJson()
            ->timeout((int) config('services.nac.timeout', 10));
    }

    protected function longRunningClient(): PendingRequest
    {
        return $this->baseClient()
            ->acceptJson()
            ->timeout((int) config('services.nac.long_timeout', 120));
    }

    protected function baseClient(): PendingRequest
    {
        return Http::baseUrl(rtrim((string) config('services.nac.base_url'), '/'))
            ->connectTimeout((int) config('services.nac.connect_timeout', 3))
            ->retry(
                max(1, (int) config('services.nac.retry_times', 2)),
                max(0, (int) config('services.nac.retry_sleep_ms', 200)),
                null,
                false
            );
    }

    public function devices(): array
    {
        return $this->get('/api/v1/devices');
    }

    public function devicesBySwitch(string $switchId, ?int $ifIndex = null): array
    {
        $query = [
            'switch_id' => trim($switchId),
        ];

        if ($ifIndex !== null) {
            $query['if_index'] = $ifIndex;
        }

        try {
            $response = $this->client()->get('/api/v1/devices', $query);
        } catch (Throwable $e) {
            Log::warning('Optional NAC API request failed', [
                'uri' => '/api/v1/devices',
                'query' => $query,
                'message' => $e->getMessage(),
            ]);

            return [];
        }

        if (! $response->successful()) {
            Log::warning('Optional NAC API request failed', [
                'uri' => '/api/v1/devices',
                'query' => $query,
                'status' => $response->status(),
            ]);

            return [];
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    public function device(string $macAddress): array
    {
        return $this->get('/api/v1/devices/'.rawurlencode($macAddress));
    }

    public function approveDevice(string $macAddress, array $payload): array
    {
        return $this->post('/api/v1/devices/'.rawurlencode($macAddress).'/approve', $payload);
    }

    public function blockDevice(string $macAddress, array $payload = []): array
    {
        return $this->post('/api/v1/devices/'.rawurlencode($macAddress).'/block', $payload);
    }

    public function retireDevice(string $macAddress, array $payload = []): array
    {
        return $this->post('/api/v1/devices/'.rawurlencode($macAddress).'/retire', $payload);
    }

    public function createIdentitySnapshot(string $macAddress, array $payload): array
    {
        return $this->post('/api/v1/devices/'.rawurlencode($macAddress).'/identity-snapshots', $payload);
    }

    public function portEvents(int $limit = 20): array
    {
        return $this->optionalGet('/api/v1/port-events?limit='.max(1, $limit));
    }

    public function auditLogs(int $limit = 20): array
    {
        return $this->optionalGet('/api/v1/audit-logs?limit='.max(1, $limit));
    }

    public function switches(): array
    {
        return $this->get('/api/v1/switches');
    }

    public function resolveSwitch(?string $name = null, ?string $managementIp = null): array
    {
        $query = array_filter([
            'name' => filled($name) ? trim((string) $name) : null,
            'management_ip' => filled($managementIp) ? trim((string) $managementIp) : null,
        ], fn ($value) => $value !== null && $value !== '');

        if ($query === []) {
            return [];
        }

        $response = $this->client()->get('/api/v1/switches/resolve', $query);

        if ($response->status() === 404) {
            return [];
        }

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: /api/v1/switches/resolve ({$response->status()})");
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    public function switchLive(string $id): array
    {
        return $this->get("/api/v1/switches/{$id}/live");
    }

    public function switchPorts(string $id): array
    {
        return $this->get("/api/v1/switches/{$id}/ports");
    }

    public function switchPortSummary(string $id): array
    {
        return $this->get("/api/v1/switches/{$id}/ports/summary");
    }

    public function executeSNMPPortVLAN(array $payload): array
    {
        return $this->post('/api/v1/enforcement/snmp-port-vlan', $payload);
    }

    public function executeSSHPortVLAN(array $payload): array
    {
        return $this->post('/api/v1/enforcement/ssh-port-vlan', $payload);
    }

    public function createDiscoveryJob(array $payload): array
    {
        return $this->post('/api/v1/discovery/jobs', $payload);
    }

    public function discoveryJob(string $id): array
    {
        return $this->get("/api/v1/discovery/jobs/{$id}");
    }

    public function dispatchDiscoveryJob(string $id, ?string $workerId = null): array
    {
        $payload = [];
        if (filled($workerId)) {
            $payload['worker_id'] = trim((string) $workerId);
        }

        $response = $this->client()->post("/api/v1/discovery/jobs/{$id}/dispatch", $payload);

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: /api/v1/discovery/jobs/{$id}/dispatch ({$response->status()})");
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    public function runDiscoveryJob(string $id, ?string $workerId = null): array
    {
        $payload = [];
        if (filled($workerId)) {
            $payload['worker_id'] = trim((string) $workerId);
        }

        $response = $this->longRunningClient()->post("/api/v1/discovery/jobs/{$id}/run", $payload);

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: /api/v1/discovery/jobs/{$id}/run ({$response->status()})");
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    public function observations(): array
    {
        return $this->get('/api/v1/mac-observations');
    }

    public function topologyLinks(): array
    {
        return $this->get('/api/v1/topology-links');
    }

    public function enforcementDecisions(): array
    {
        return $this->get('/api/v1/enforcement-decisions');
    }

    public function approveEnforcementDecision(string $id): void
    {
        $this->postNoContent("/api/v1/enforcement-decisions/{$id}/approve");
    }

    public function rejectEnforcementDecision(string $id): void
    {
        $this->postNoContent("/api/v1/enforcement-decisions/{$id}/reject");
    }

    public function retryEnforcementDecision(string $id): void
    {
        $this->postNoContent("/api/v1/enforcement-decisions/{$id}/retry");
    }

    public function policies(): array
    {
        return $this->get('/api/v1/policies');
    }

    public function createPolicy(array $payload): array
    {
        return $this->post('/api/v1/policies', $payload);
    }

    public function disablePolicy(string $id): void
    {
        $response = $this->client()->post("/api/v1/policies/{$id}/disable");

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: /api/v1/policies/{$id}/disable ({$response->status()})");
        }
    }

    public function dashboard(): array
    {
        $devices = $this->devices();
        $switches = $this->switches();
        $observations = $this->optionalGet('/api/v1/mac-observations');
        $topologyLinks = $this->optionalGet('/api/v1/topology-links');
        $policies = $this->optionalGet('/api/v1/policies');
        $enforcementDecisions = $this->optionalGet('/api/v1/enforcement-decisions');

        return [
            'devices' => $devices,
            'switches' => $switches,
            'observations' => $observations,
            'topologyLinks' => $topologyLinks,
            'policies' => $policies,
            'enforcementDecisions' => $enforcementDecisions,
            'stats' => [
                'device_count' => count($devices),
                'switch_count' => count($switches),
                'observation_count' => count($observations),
                'topology_link_count' => count($topologyLinks),
                'policy_count' => count($policies),
                'enforcement_count' => count($enforcementDecisions),
                'active_devices' => count(array_filter($devices, fn ($device) => ($device['status'] ?? '') === 'active')),
            ],
        ];
    }

    protected function get(string $uri, ?PendingRequest $client = null): array
    {
        try {
            $response = ($client ?? $this->client())->get($uri);
        } catch (Throwable $e) {
            throw new RuntimeException("NAC API connection failed: {$uri} ({$e->getMessage()})", 0, $e);
        }

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: {$uri} ({$response->status()})");
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    protected function post(string $uri, array $payload): array
    {
        try {
            $response = $this->client()->post($uri, $payload);
        } catch (Throwable $e) {
            throw new RuntimeException("NAC API connection failed: {$uri} ({$e->getMessage()})", 0, $e);
        }

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: {$uri} ({$response->status()})");
        }

        $decoded = $response->json();

        return is_array($decoded) ? $decoded : [];
    }

    protected function postNoContent(string $uri): void
    {
        try {
            $response = $this->client()->post($uri);
        } catch (Throwable $e) {
            throw new RuntimeException("NAC API connection failed: {$uri} ({$e->getMessage()})", 0, $e);
        }

        if (! $response->successful()) {
            throw new RuntimeException("NAC API request failed: {$uri} ({$response->status()})");
        }
    }

    protected function optionalGet(string $uri, ?PendingRequest $client = null): array
    {
        try {
            return $this->get($uri, $client);
        } catch (RuntimeException $e) {
            Log::warning('Optional NAC API request failed', [
                'uri' => $uri,
                'message' => $e->getMessage(),
            ]);

            return [];
        }
    }
}
