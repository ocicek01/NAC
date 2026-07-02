<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\SwitchPort;
use App\Services\AuditLogService;
use App\Services\PortActionService;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Illuminate\Validation\ValidationException;
use Throwable;

class NacActionController extends Controller
{
    public function __construct(
        protected PortActionService $portActionService,
        protected AuditLogService $auditLogService
    ) {
    }

    public function store(Request $request, SwitchPort $port): JsonResponse
    {
        try {
            $result = $this->portActionService->execute(
                $port,
                $request->all(),
                $this->auditLogService->contextFromRequest($request)
            );

            return response()->json($result);
        } catch (ValidationException $e) {
            return response()->json([
                'message' => $e->getMessage(),
                'errors' => $e->errors(),
            ], 422);
        } catch (Throwable $e) {
            report($e);

            return response()->json([
                'message' => $e->getMessage() !== '' ? $e->getMessage() : 'Port aksiyonu uygulanamadi.',
            ], 500);
        }
    }
}
