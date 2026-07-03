<?php

namespace App\Http\Controllers\Api;

use App\Http\Controllers\Controller;
use App\Services\PortStatusUpdater;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\StreamedResponse;

class PortStatusEventController extends Controller
{
    public function __construct(
        protected PortStatusUpdater $portStatusUpdater,
    ) {
    }

    public function stream(Request $request): StreamedResponse
    {
        $lastEventId = (string) ($request->header('Last-Event-ID') ?: $request->query('last_event_id', ''));
        $retryMs = max(1000, (int) config('services.switch_port_status.sse_retry_ms', 3000));

        return response()->stream(function () use ($lastEventId, $retryMs) {
            $currentLastEventId = $lastEventId;
            $startedAt = time();

            echo 'retry: '.$retryMs."\n\n";
            @ob_flush();
            flush();

            while (! connection_aborted() && (time() - $startedAt) < 25) {
                $events = $this->portStatusUpdater->eventsAfter($currentLastEventId);

                if ($events !== []) {
                    foreach ($events as $event) {
                        $currentLastEventId = (string) ($event['id'] ?? $currentLastEventId);
                        echo 'id: '.$currentLastEventId."\n";
                        echo "event: port_status_changed\n";
                        echo 'data: '.json_encode($event, JSON_UNESCAPED_SLASHES)."\n\n";
                    }
                } else {
                    echo ': heartbeat '.now()->toIso8601String()."\n\n";
                }

                @ob_flush();
                flush();
                sleep(1);
            }
        }, 200, [
            'Content-Type' => 'text/event-stream',
            'Cache-Control' => 'no-cache, no-transform',
            'Connection' => 'keep-alive',
            'X-Accel-Buffering' => 'no',
        ]);
    }
}
