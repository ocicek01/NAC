<?php

namespace App\Http\Controllers\Web;

use App\Http\Controllers\Controller;
use App\Models\NetworkSwitch;
use App\Models\Zone;
use App\Services\AuditLogService;
use App\Services\SwitchStatsService;
use App\Services\ZoneStatsService;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Validator;
use Illuminate\Validation\Rule;

class SwitchPageController extends Controller
{
    public function __construct(
        protected ZoneStatsService $zoneStatsService,
        protected SwitchStatsService $switchStatsService,
        protected AuditLogService $auditLogService
    ) {
    }

    public function index()
    {
        $zones = $this->zoneStatsService->getZoneCollection();
        $zoneCards = $zones->map(fn (Zone $zone) => $this->zoneStatsService->zoneCard($zone))->values();

        return view('menu-preview', [
            'summaryCards' => $this->zoneStatsService->summaryCards($zones),
            'zones' => $zoneCards,
        ]);
    }

    public function create(Request $request)
    {
        return view('switches.create', [
            'zoneOptions' => Zone::query()->orderBy('name')->get(['slug', 'name'])->map(fn (Zone $zone) => [
                'slug' => $zone->slug,
                'label' => $zone->name,
            ])->values()->all(),
            'selectedZone' => $request->query('zone'),
        ]);
    }

    public function store(Request $request): RedirectResponse
    {
        $validated = Validator::make($request->all(), [
            'hostname' => ['required', 'string', 'max:255', 'unique:switches,hostname'],
            'ip_address' => ['required', 'ip', 'unique:switches,ip_address'],
            'vendor' => ['required', 'string', 'max:255'],
            'model' => ['required', 'string', 'max:255'],
            'zone' => ['required', 'string', Rule::exists('zones', 'slug')],
            'port_count' => ['required', 'integer', 'between:1,128'],
            'location' => ['nullable', 'string', 'max:255'],
            'status' => ['required', Rule::in(['online', 'offline', 'warning', 'unmanaged'])],
            'managed' => ['required', 'boolean'],
            'description' => ['nullable', 'string'],
        ], [
            'hostname.required' => 'Hostname alani zorunludur.',
            'ip_address.required' => 'IP Address alani zorunludur.',
            'ip_address.ip' => 'Gecerli bir IP adresi giriniz.',
            'vendor.required' => 'Vendor alani zorunludur.',
            'model.required' => 'Model alani zorunludur.',
            'zone.required' => 'Zone alani zorunludur.',
            'port_count.required' => 'Port sayisi zorunludur.',
            'port_count.integer' => 'Port sayisi sayisal olmalidir.',
            'port_count.between' => 'Port sayisi 1 ile 128 arasinda olmalidir.',
        ])->validate();

        $zone = Zone::query()->where('slug', $validated['zone'])->firstOrFail();

        DB::transaction(function () use ($validated, $request, $zone) {
            $switch = NetworkSwitch::create([
                'zone_id' => $zone->id,
                'hostname' => $validated['hostname'],
                'ip_address' => $validated['ip_address'],
                'vendor' => $validated['vendor'],
                'model' => $validated['model'],
                'location' => $validated['location'] ?? null,
                'status' => $validated['status'],
                'managed' => (bool) $validated['managed'],
                'nac_mode' => 'monitor',
                'port_count' => $validated['port_count'],
                'last_seen_at' => now(),
            ]);

            for ($index = 1; $index <= $switch->port_count; $index++) {
                $switch->ports()->create([
                    'port_index' => $index,
                    'port_name' => 'Gi1/0/'.$index,
                    'status' => $index <= 2 ? 'up' : 'down',
                    'port_type' => $index === 1 ? 'trunk' : ($index === 2 ? 'uplink' : 'access'),
                    'nac_mode' => 'inherit',
                    'vlan_id' => 1,
                    'speed' => $index <= 2 ? '1 Gbps' : '0',
                    'duplex' => 'Full',
                    'poe_enabled' => false,
                    'poe_power' => 0,
                    'last_change_at' => now(),
                ]);
            }

            $this->auditLogService->log('switch_created_from_web', 'switch', $switch->id, array_merge(
                $this->auditLogService->contextFromRequest($request),
                [
                    'switch_id' => $switch->id,
                    'new_value' => [
                        'hostname' => $switch->hostname,
                        'zone' => $zone->slug,
                    ],
                ]
            ));
        });

        $message = $request->input('submit_action') === 'save_test'
            ? 'Switch kaydi dogrulandi. Test islemi daha sonra eklenecek.'
            : 'Switch kaydi basariyla olusturuldu.';

        return redirect()
            ->route('switches.create', ['zone' => $validated['zone']])
            ->with('success', $message);
    }

    public function zone(Zone $zone)
    {
        $zone->loadMissing('switches.ports.currentLocation.endpoint');
        $detail = $this->zoneStatsService->zoneDetail($zone);

        return view('zone-detail', [
            'zone' => array_merge($detail['zone'], $detail['view']),
            'zoneSlug' => $zone->slug,
        ]);
    }

    public function show(Zone $zone, string $switch)
    {
        $switchModel = NetworkSwitch::query()
            ->where('zone_id', $zone->id)
            ->whereRaw('lower(hostname) = ?', [strtolower($switch)])
            ->with(['zone', 'ports.currentLocation.endpoint'])
            ->firstOrFail();

        $detail = $this->switchStatsService->detail($switchModel, true);

        return view('switch-detail', [
            'switchData' => $detail['view'],
            'zoneSlug' => $zone->slug,
            'switchSlug' => strtolower($switchModel->hostname),
        ]);
    }
}

