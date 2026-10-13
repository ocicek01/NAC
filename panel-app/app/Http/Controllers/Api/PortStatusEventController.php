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
        if (config('services.nac.port_observer_enabled', false)) {
            return $this->observerStream($request);
        }
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

    protected function observerStream(Request $request): StreamedResponse
    {
        $request->validate(['switch_id' => ['required', 'integer', 'exists:switches,id']]);
        $switch = \App\Models\NetworkSwitch::with('ports')->findOrFail($request->query('switch_id'));
        $observer = app(\App\Services\ObserverPortStatus::class);
        $remoteId = $observer->resolve($switch);
        $cursor = (string) ($request->header('Last-Event-ID') ?: $request->query('last_event_id', ''));
        abort_if($cursor !== '' && ! preg_match('/^\d{1,20}_[0-9a-fA-F-]{36}$/D', $cursor), 422, 'Invalid event cursor');
        // Establish the cursor before the browser requests its initial snapshot.
        $initial = $observer->feed($switch, $remoteId, $cursor === '' ? null : $cursor);

        return response()->stream(function () use ($switch, $remoteId, $observer, $initial) {
            $feed = $initial;
            $startedAt = time();
            $lastHeartbeat = 0;
            $lastEmittedCursor = '';
            echo "retry: 3000\nevent: observer_ready\ndata: {}\n\n";
            while (! connection_aborted() && time() - $startedAt < 25) {
                if (($feed['ports'] ?? []) !== []) {
                    echo 'id: '.$feed['cursor']."\n";
                    echo "event: port_snapshot_batch\n";
                    echo 'data: '.json_encode($feed, JSON_UNESCAPED_SLASHES)."\n\n";
                    @ob_flush();
                    flush();
                    $lastEmittedCursor = (string) $feed['cursor'];
                } elseif (($feed['cursor'] ?? '') !== '' && (string) $feed['cursor'] !== $lastEmittedCursor) {
                    echo 'id: '.$feed['cursor']."\n";
                    echo "event: observer_cursor\n";
                    echo "data: {}\n\n";
                    @ob_flush();
                    flush();
                    $lastEmittedCursor = (string) $feed['cursor'];
                } elseif (time() - $lastHeartbeat >= 10) {
                    echo ': heartbeat '.now()->toIso8601String()."\n\n";
                    @ob_flush();
                    flush();
                    $lastHeartbeat = time();
                }
                sleep(2);
                try {
                    $feed = $observer->feed($switch, $remoteId, $feed['cursor']);
                } catch (\Throwable $error) {
                    report($error);
                    break;
                }
            }
        }, 200, [
            'Content-Type' => 'text/event-stream',
            'Cache-Control' => 'no-cache, no-transform',
            'X-Accel-Buffering' => 'no',
        ]);
    }
}
