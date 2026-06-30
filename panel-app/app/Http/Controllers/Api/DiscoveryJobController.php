<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Models\NacAuditLog;
use App\Services\AuditLogService;
use App\Services\NacApiClient;
use Illuminate\Http\JsonResponse;

class DiscoveryJobController extends Controller
{
    public function __construct(
        protected NacApiClient $nacApiClient,
        protected AuditLogService $auditLogService
    ) {
    }

    public function show(string $job): JsonResponse
    {
        $payload = $this->nacApiClient->discoveryJob($job);
        $this->logTerminalStateIfNeeded($job, $payload);

        return response()->json([
            'data' => $payload,
        ]);
    }

    protected function logTerminalStateIfNeeded(string $jobId, array $payload): void
    {
        $status = strtolower(trim((string) ($payload['status'] ?? '')));
        if (! in_array($status, ['completed', 'failed'], true)) {
            return;
        }

        $requested = NacAuditLog::query()
            ->whereIn('action', ['switch_ports_rediscovery_requested', 'switch_port_rediscovery_requested'])
            ->where('new_value->job_id', $jobId)
            ->latest('created_at')
            ->first();

        if (! $requested) {
            return;
        }

        $completionAction = $requested->action === 'switch_port_rediscovery_requested'
            ? ($status === 'completed' ? 'switch_port_rediscovered' : 'switch_port_rediscovery_failed')
            : ($status === 'completed' ? 'switch_ports_rediscovered' : 'switch_ports_rediscovery_failed');

        $exists = NacAuditLog::query()
            ->where('action', $completionAction)
            ->where('new_value->job_id', $jobId)
            ->exists();

        if ($exists) {
            return;
        }

        $this->auditLogService->log($completionAction, $requested->target_type, (int) $requested->target_id, [
            'actor_id' => $requested->actor_id,
            'switch_id' => $requested->switch_id,
            'switch_port_id' => $requested->switch_port_id,
            'endpoint_id' => $requested->endpoint_id,
            'ip_address' => $requested->ip_address,
            'created_at' => now(),
            'new_value' => [
                'job_id' => $jobId,
                'status' => $payload['status'] ?? null,
                'current_step' => $payload['current_step'] ?? null,
                'progress_percent' => $payload['progress_percent'] ?? null,
                'summary' => $payload['summary'] ?? null,
                'error_message' => $payload['error_message'] ?? null,
            ],
        ]);
    }
}
