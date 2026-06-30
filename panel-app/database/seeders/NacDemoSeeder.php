<?php

namespace Database\Seeders;

use App\Models\Endpoint;
use App\Models\EndpointLocation;
use App\Models\NetworkSwitch;
use App\Models\SwitchPort;
use App\Models\Zone;
use Illuminate\Database\Seeder;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Str;

class NacDemoSeeder extends Seeder
{
    public function run(): void
    {
        DB::transaction(function () {
            $zones = [
                ['name' => 'Rektorluk', 'status' => 'normal', 'description' => 'Merkezi idari binalar'],
                ['name' => 'Cumhuriyet MYO', 'status' => 'warning', 'description' => 'Meslek yuksekokulu binasi'],
                ['name' => 'Muhendislik Fakultesi', 'status' => 'normal', 'description' => 'Fakulte ve laboratuvarlar'],
                ['name' => 'Kutuphane', 'status' => 'normal', 'description' => 'Kutuphane ve ortak alanlar'],
            ];

            foreach ($zones as $zoneIndex => $zoneData) {
                $zone = Zone::firstOrCreate(
                    ['slug' => Str::slug($zoneData['name'])],
                    $zoneData
                );

                foreach ([48, 24, 48] as $switchOffset => $portCount) {
                    $switchNumber = $switchOffset + 1;
                    $switch = NetworkSwitch::create([
                        'zone_id' => $zone->id,
                        'hostname' => strtoupper(Str::substr(Str::slug($zone->name), 0, 3)).'-SW-'.str_pad((string) $switchNumber, 2, '0', STR_PAD_LEFT),
                        'ip_address' => '10.10.'.($zoneIndex + 1).'.'.(10 + $switchOffset),
                        'vendor' => ['Aruba', 'Cisco', 'Fortinet'][$switchOffset % 3],
                        'model' => $portCount === 48 ? ['6200F', 'Catalyst 9300', 'FortiSwitch 248E'][$switchOffset % 3] : '6100',
                        'location' => $zone->name.' Kat '.($switchOffset + 1),
                        'status' => $zoneIndex === 1 && $switchOffset === 1 ? 'offline' : ($switchOffset === 2 ? 'warning' : 'online'),
                        'managed' => ! ($zoneIndex === 3 && $switchOffset === 2),
                        'nac_mode' => $switchOffset === 0 ? 'enforcement' : ($switchOffset === 1 ? 'monitor' : 'disabled'),
                        'port_count' => $portCount,
                        'last_seen_at' => Carbon::now()->subSeconds(($zoneIndex + 1) * ($switchOffset + 2) * 15),
                        'created_at' => now()->subDays(20 - $zoneIndex)->subHours($switchOffset * 8),
                        'updated_at' => now(),
                    ]);

                    for ($portIndex = 1; $portIndex <= $portCount; $portIndex++) {
                        $status = match (true) {
                            $portIndex % 16 === 0 => 'disabled',
                            $portIndex % 11 === 0 => 'down',
                            default => 'up',
                        };

                        $portType = match (true) {
                            $portIndex === 1 => 'trunk',
                            $portIndex === 2 => 'uplink',
                            $portIndex % 13 === 0 => 'printer',
                            $portIndex % 9 === 0 => 'ap',
                            $portIndex % 7 === 0 => 'server',
                            $portIndex % 17 === 0 => 'camera',
                            default => 'access',
                        };

                        $nacMode = match (true) {
                            $portIndex % 10 === 0 => 'monitor',
                            $portIndex % 8 === 0 => 'enforcement',
                            default => 'inherit',
                        };

                        $port = SwitchPort::create([
                            'switch_id' => $switch->id,
                            'port_index' => $portIndex,
                            'port_name' => 'Gi1/0/'.$portIndex,
                            'status' => $status,
                            'port_type' => $portType,
                            'nac_mode' => $nacMode,
                            'vlan_id' => $portType === 'trunk' ? 100 : 10,
                            'speed' => $status === 'down' ? '0' : ($portType === 'printer' ? '100 Mbps' : '1 Gbps'),
                            'duplex' => 'Full',
                            'poe_enabled' => in_array($portType, ['ap', 'camera'], true),
                            'poe_power' => in_array($portType, ['ap', 'camera'], true) ? 15.4 : 0,
                            'last_change_at' => now()->subMinutes(($portIndex + $switchOffset) % 45),
                        ]);

                        if ($status !== 'up' || in_array($portType, ['trunk', 'uplink'], true) || $portIndex % 3 === 0) {
                            continue;
                        }

                        $endpointStatus = match (true) {
                            $portIndex % 23 === 0 => 'unauthorized',
                            $portIndex % 19 === 0 => 'quarantine',
                            $portIndex % 14 === 0 => 'guest',
                            default => 'authenticated',
                        };

                        $policy = match ($endpointStatus) {
                            'guest' => 'Guest',
                            'quarantine' => 'Quarantine',
                            'unauthorized' => 'Reject',
                            default => 'Employee',
                        };

                        $role = match ($endpointStatus) {
                            'guest' => 'Guest',
                            'quarantine', 'unauthorized' => 'Unauthorized',
                            default => $portIndex % 5 === 0 ? 'IoT' : ($portIndex % 4 === 0 ? 'Student' : 'Employee'),
                        };

                        $vlanId = match ($endpointStatus) {
                            'guest' => 30,
                            'quarantine' => 998,
                            'unauthorized' => 999,
                            default => 10,
                        };

                        $endpoint = Endpoint::create([
                            'mac_address' => sprintf('52:BC:%02X:%02X:%02X:%02X', $zone->id, $switch->id % 255, $portIndex, ($portIndex * 2) % 255),
                            'ip_address' => '10.'.($zoneIndex + 10).'.'.($switchOffset + 1).'.'.$portIndex,
                            'hostname' => strtoupper(Str::substr($zone->slug, 0, 3)).'-EP-'.str_pad((string) $portIndex, 2, '0', STR_PAD_LEFT),
                            'user_name' => strtolower(Str::ascii(Str::substr($zone->slug, 0, 3))).'.user'.$portIndex,
                            'device_type' => match (true) {
                                $portType === 'ap' => 'Access Point',
                                $portType === 'printer' => 'Printer',
                                $portType === 'camera' => 'Camera',
                                $portType === 'server' => 'Server',
                                default => 'Windows',
                            },
                            'policy_name' => $policy,
                            'role_name' => $role,
                            'vlan_id' => $vlanId,
                            'status' => $endpointStatus,
                            'last_seen_at' => now()->subMinutes($portIndex % 10),
                        ]);

                        EndpointLocation::create([
                            'endpoint_id' => $endpoint->id,
                            'switch_id' => $switch->id,
                            'switch_port_id' => $port->id,
                            'vlan_id' => $vlanId,
                            'first_seen_at' => now()->subHours(6)->subMinutes($portIndex),
                            'last_seen_at' => now()->subMinutes($portIndex % 10),
                        ]);
                    }
                }
            }
        });
    }
}
