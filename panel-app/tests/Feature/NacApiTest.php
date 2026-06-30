<?php

namespace Tests\Feature;

use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Services\NacApiClient;
use App\Models\Zone;
use Database\Seeders\NacDemoSeeder;
use Illuminate\Foundation\Testing\RefreshDatabase;
use Tests\TestCase;

class NacApiTest extends TestCase
{
    use RefreshDatabase;

    public function test_required_api_endpoints_work(): void
    {
        $this->seed(NacDemoSeeder::class);

        $this->mock(NacApiClient::class, function ($mock): void {
            $mock->shouldReceive('topologyLinks')->andReturn([]);
            $mock->shouldReceive('resolveSwitch')->andReturn([]);
            $mock->shouldReceive('switches')->andReturn([]);
            $mock->shouldReceive('discoveryJob')->andReturn([]);
        });

        $zone = Zone::query()->firstOrFail();
        $switch = NetworkSwitch::query()->with('ports')->firstOrFail();
        $port = SwitchPort::query()
            ->whereNotIn('port_type', ['trunk', 'uplink'])
            ->firstOrFail();

        $this->getJson('/api/zones')
            ->assertOk()
            ->assertJsonStructure(['data' => [['id', 'name', 'slug', 'switch_count', 'health_status']]]);

        $this->getJson('/api/zones/'.$zone->id)
            ->assertOk()
            ->assertJsonStructure(['zone', 'kpis', 'switches', 'endpoint_distribution', 'port_distribution', 'recent_alerts']);

        $this->getJson('/api/switches/'.$switch->id)
            ->assertOk()
            ->assertJsonStructure(['switch', 'kpis', 'port_map', 'side_panel', 'recent_events']);

        $this->getJson('/api/switches/'.$switch->id.'/ports')
            ->assertOk()
            ->assertJsonStructure(['data' => [['id', 'label', 'state']]]);

        $this->putJson('/api/switches/'.$switch->id.'/nac-mode', ['nac_mode' => 'monitor'])
            ->assertOk()
            ->assertJsonPath('data.nac_mode', 'monitor');

        $this->putJson('/api/switch-ports/'.$port->id.'/nac-mode', ['nac_mode' => 'enforcement'])
            ->assertOk()
            ->assertJsonPath('data.nac_mode', 'enforcement');

        $this->postJson('/api/switch-ports/'.$port->id.'/actions', ['action' => 'force_reauth'])
            ->assertOk()
            ->assertJsonPath('status', 'success');
    }

    public function test_protected_port_actions_are_blocked(): void
    {
        $this->seed(NacDemoSeeder::class);

        $port = SwitchPort::query()->where('port_type', 'uplink')->firstOrFail();

        $this->postJson('/api/switch-ports/'.$port->id.'/actions', ['action' => 'disable_port'])
            ->assertStatus(422)
            ->assertJsonValidationErrors('action');
    }

    public function test_switch_port_rediscovery_endpoint_dispatches_full_switch_job(): void
    {
        $this->seed(NacDemoSeeder::class);

        $switch = NetworkSwitch::query()->firstOrFail();

        $this->mock(NacApiClient::class, function ($mock) use ($switch): void {
            $mock->shouldReceive('topologyLinks')->andReturn([]);
            $mock->shouldReceive('resolveSwitch')
                ->atLeast()->once()
                ->withArgs(fn (?string $hostname, ?string $managementIp) => $hostname === $switch->hostname && $managementIp === $switch->ip_address)
                ->andReturn([
                    'id' => 'go-switch-1',
                    'name' => $switch->hostname,
                    'management_ip' => $switch->ip_address,
                ]);
            $mock->shouldReceive('createDiscoveryJob')
                ->once()
                ->withArgs(fn (array $payload) => ($payload['switch_id'] ?? null) === 'go-switch-1' && ($payload['scope'] ?? null) === 'full')
                ->andReturn([
                    'id' => 'job-1',
                    'switch_id' => 'go-switch-1',
                    'scope' => 'full',
                    'status' => 'queued',
                ]);
            $mock->shouldReceive('dispatchDiscoveryJob')
                ->once()
                ->withArgs(fn (string $id, ?string $workerId) => $id === 'job-1' && $workerId === 'panel-web')
                ->andReturn([
                    'id' => 'job-1',
                    'switch_id' => 'go-switch-1',
                    'scope' => 'full',
                    'status' => 'running',
                    'current_step' => 'claimed',
                    'progress_percent' => 5,
                ]);
            $mock->shouldReceive('switchPortSummary')->andReturn([]);
            $mock->shouldReceive('switchPorts')->andReturn([]);
            $mock->shouldReceive('switches')->andReturn([]);
            $mock->shouldReceive('discoveryJob')->andReturn([]);
        });

        $this->postJson('/api/switches/'.$switch->id.'/rediscover-ports')
            ->assertStatus(202)
            ->assertJsonPath('message', 'Tum portlar icin tarama baslatildi.')
            ->assertJsonPath('data.job.id', 'job-1')
            ->assertJsonPath('data.switch_id', $switch->id)
            ->assertJsonStructure(['data' => ['job', 'go_switch_id', 'switch_id']]);
    }

    public function test_switch_port_rediscovery_endpoint_dispatches_parent_switch_job(): void
    {
        $this->seed(NacDemoSeeder::class);

        $port = SwitchPort::query()->where('port_type', 'access')->firstOrFail();
        $switch = $port->switch()->firstOrFail();

        $this->mock(NacApiClient::class, function ($mock) use ($port, $switch): void {
            $mock->shouldReceive('resolveSwitch')
                ->once()
                ->withArgs(fn (?string $hostname, ?string $managementIp) => $hostname === $switch->hostname && $managementIp === $switch->ip_address)
                ->andReturn([
                    'id' => 'go-switch-1',
                    'name' => $switch->hostname,
                    'management_ip' => $switch->ip_address,
                ]);
            $mock->shouldReceive('createDiscoveryJob')
                ->once()
                ->withArgs(fn (array $payload) => ($payload['switch_id'] ?? null) === 'go-switch-1' && ($payload['scope'] ?? null) === 'full')
                ->andReturn([
                    'id' => 'job-2',
                    'switch_id' => 'go-switch-1',
                    'scope' => 'full',
                    'status' => 'queued',
                ]);
            $mock->shouldReceive('dispatchDiscoveryJob')
                ->once()
                ->withArgs(fn (string $id, ?string $workerId) => $id === 'job-2' && $workerId === 'panel-web')
                ->andReturn([
                    'id' => 'job-2',
                    'switch_id' => 'go-switch-1',
                    'scope' => 'full',
                    'status' => 'running',
                ]);
            $mock->shouldReceive('topologyLinks')->andReturn([]);
            $mock->shouldReceive('switchPortSummary')->andReturn([]);
            $mock->shouldReceive('switchPorts')->andReturn([]);
            $mock->shouldReceive('switches')->andReturn([]);
            $mock->shouldReceive('discoveryJob')->andReturn([]);
        });

        $this->postJson('/api/switch-ports/'.$port->id.'/rediscover')
            ->assertStatus(202)
            ->assertJsonPath('message', 'Port taramasi baslatildi.')
            ->assertJsonPath('data.job.id', 'job-2')
            ->assertJsonPath('data.switch_id', $switch->id)
            ->assertJsonPath('data.selected_port_id', $port->id);
    }
}
