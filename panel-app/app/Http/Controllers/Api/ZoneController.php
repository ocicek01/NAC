<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\Zone;
use App\Services\ZoneStatsService;
use Illuminate\Http\JsonResponse;

class ZoneController extends Controller
{
    public function __construct(
        protected ZoneStatsService $zoneStatsService
    ) {
    }

    public function index(): JsonResponse
    {
        $zones = $this->zoneStatsService->getZoneCollection()
            ->map(fn (Zone $zone) => $this->zoneStatsService->zoneCard($zone))
            ->values();

        return response()->json([
            'data' => $zones,
        ]);
    }

    public function show(Zone $zone): JsonResponse
    {
        $zone->loadMissing('switches.ports.currentLocation.endpoint');

        return response()->json($this->zoneStatsService->zoneDetail($zone));
    }
}
