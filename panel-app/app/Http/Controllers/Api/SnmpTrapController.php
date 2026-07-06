<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Services\SnmpTrapIngestService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Validator;

class SnmpTrapController extends Controller
{
    public function __construct(
        protected SnmpTrapIngestService $snmpTrapIngestService,
    ) {
    }

    public function store(Request $request): JsonResponse
    {
        $configuredToken = trim((string) config('services.nac.trap_ingest_token', ''));
        $providedToken = trim((string) $request->header('X-TRAP-TOKEN', ''));

        if ($configuredToken !== '' && ! hash_equals($configuredToken, $providedToken)) {
            return response()->json([
                'message' => 'Gecersiz trap token.',
            ], 403);
        }

        $payload = Validator::make($request->all(), [
            'switch_ip' => ['nullable', 'ip'],
            'source_ip' => ['nullable', 'ip'],
            'switch_hostname' => ['nullable', 'string', 'max:255'],
            'if_index' => ['required', 'integer', 'min:1'],
            'if_name' => ['nullable', 'string', 'max:255'],
            'if_descr' => ['nullable', 'string', 'max:255'],
            'admin_status' => ['nullable'],
            'oper_status' => ['nullable'],
            'speed' => ['nullable', 'string', 'max:255'],
            'trap_oid' => ['nullable', 'string', 'max:255'],
            'trap_type' => ['nullable', 'string', 'max:255'],
            'occurred_at' => ['nullable', 'date'],
            'varbinds' => ['nullable', 'array'],
        ])->validate();

        $result = $this->snmpTrapIngestService->ingest($payload, $request->ip());

        return response()->json([
            'message' => 'SNMP trap alindi.',
            'data' => $result,
        ], 202);
    }
}
