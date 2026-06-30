<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\SwitchPort;
use App\Services\AuditLogService;
use App\Services\PortActionService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

class NacActionController extends Controller
{
    public function __construct(
        protected PortActionService $portActionService,
        protected AuditLogService $auditLogService
    ) {
    }

    public function store(Request $request, SwitchPort $port): JsonResponse
    {
        $result = $this->portActionService->execute(
            $port,
            $request->all(),
            $this->auditLogService->contextFromRequest($request)
        );

        return response()->json($result);
    }
}
