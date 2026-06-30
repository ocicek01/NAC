<?php

namespace App\Services;

use App\Models\NacAuditLog;
use Illuminate\Http\Request;
use Illuminate\Support\Carbon;

class AuditLogService
{
    public function log(string $action, string $targetType, int $targetId, array $context = []): NacAuditLog
    {
        return NacAuditLog::create([
            'actor_id' => $context['actor_id'] ?? null,
            'action' => $action,
            'target_type' => $targetType,
            'target_id' => $targetId,
            'switch_id' => $context['switch_id'] ?? null,
            'switch_port_id' => $context['switch_port_id'] ?? null,
            'endpoint_id' => $context['endpoint_id'] ?? null,
            'old_value' => $context['old_value'] ?? null,
            'new_value' => $context['new_value'] ?? null,
            'ip_address' => $context['ip_address'] ?? null,
            'created_at' => $context['created_at'] ?? Carbon::now(),
        ]);
    }

    public function contextFromRequest(Request $request): array
    {
        return [
            'actor_id' => optional($request->user())->id,
            'ip_address' => $request->ip(),
        ];
    }
}
