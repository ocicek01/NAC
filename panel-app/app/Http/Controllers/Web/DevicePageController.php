<?php

namespace App\Http\Controllers\Web;

use App\Http\Controllers\Controller;
use App\Services\NacApiClient;
use Illuminate\Http\RedirectResponse;
use Illuminate\Http\Request;
use RuntimeException;

class DevicePageController extends Controller
{
    public function __construct(
        protected NacApiClient $nacApiClient
    ) {
    }

    public function index()
    {
        $devices = collect($this->nacApiClient->devices())
            ->sortBy([
                fn (array $device) => $device['current_switch_name'] ?? '',
                fn (array $device) => $device['current_if_index'] ?? 0,
            ])
            ->values();

        return view('devices.index', [
            'devices' => $devices,
        ]);
    }

    public function approve(Request $request, string $mac): RedirectResponse
    {
        $targetVLAN = $this->allowVlanForRequest($request);
        $payload = [
            'approved_by' => 'panel',
            'target_vlan' => $targetVLAN,
        ];

        if ($request->filled('expires_at')) {
            $payload['expires_at'] = (string) $request->input('expires_at');
        }

        try {
            $this->nacApiClient->approveDevice($mac, $payload);
            return back()->with('success', 'Cihaz izinli duruma alindi.');
        } catch (RuntimeException $e) {
            return back()->with('error', $e->getMessage());
        }
    }

    public function block(string $mac): RedirectResponse
    {
        try {
            $this->nacApiClient->blockDevice($mac, ['approved_by' => 'panel']);
            return back()->with('success', 'Cihaz karantinaya alindi.');
        } catch (RuntimeException $e) {
            return back()->with('error', $e->getMessage());
        }
    }

    public function retire(string $mac): RedirectResponse
    {
        try {
            $this->nacApiClient->retireDevice($mac, ['approved_by' => 'panel']);
            return back()->with('success', 'Cihaz retired durumuna alindi.');
        } catch (RuntimeException $e) {
            return back()->with('error', $e->getMessage());
        }
    }

    public function guest(Request $request, string $mac): RedirectResponse
    {
        $guestVLAN = (int) config('services.nac.guest_vlan', 300);

        try {
            $this->nacApiClient->createIdentitySnapshot($mac, [
                'identity_type' => 'misafir',
                'identity_source' => 'panel_guest',
                'external_id' => $request->input('guest_identifier', 'panel-guest'),
                'username' => $request->input('guest_identifier', 'panel-guest'),
                'full_name' => $request->input('guest_full_name', 'Panel Guest Access'),
                'attributes' => [
                    'requested_by' => 'panel',
                ],
                'verified_at' => now()->toRfc3339String(),
            ]);

            $this->nacApiClient->approveDevice($mac, [
                'approved_by' => 'panel',
                'target_vlan' => (int) $request->input('target_vlan', $guestVLAN),
            ]);

            return back()->with('success', 'Cihaz guest VLAN icin yetkilendirildi.');
        } catch (RuntimeException $e) {
            return back()->with('error', $e->getMessage());
        }
    }

    protected function allowVlanForRequest(Request $request): int
    {
        if ($request->filled('target_vlan')) {
            return (int) $request->input('target_vlan');
        }

        $identityType = strtolower(trim((string) $request->input('identity_type', '')));

        return match ($identityType) {
            'ogrenci', 'misafir' => (int) config('services.nac.guest_vlan', 300),
            default => (int) config('services.nac.default_allow_vlan', 106),
        };
    }
}
