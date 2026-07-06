<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use Illuminate\Support\Facades\Log;
use Illuminate\Validation\ValidationException;
use RuntimeException;

class SnmpTrapListenerService
{
    public function __construct(
        protected SnmpTrapPacketDecoder $packetDecoder,
        protected SnmpTrapIngestService $snmpTrapIngestService,
    ) {
    }

    public function listen(?string $host = null, ?int $port = null, ?int $maxPackets = null, ?callable $onPacket = null): int
    {
        if (! extension_loaded('sockets')) {
            throw new RuntimeException('PHP sockets extension gerekli.');
        }

        if (! (bool) config('services.nac.trap_listener_enabled', true)) {
            throw new RuntimeException('SNMP trap listener devre disi.');
        }

        $host ??= (string) config('services.nac.trap_listener_host', '0.0.0.0');
        $port ??= (int) config('services.nac.trap_listener_port', 9162);
        $bufferBytes = max(2048, (int) config('services.nac.trap_listener_buffer_bytes', 65535));
        $maxPackets = $maxPackets !== null && $maxPackets > 0 ? $maxPackets : null;

        $socket = socket_create(AF_INET, SOCK_DGRAM, SOL_UDP);
        if ($socket === false) {
            throw new RuntimeException('UDP socket olusturulamadi.');
        }

        socket_set_option($socket, SOL_SOCKET, SO_REUSEADDR, 1);
        socket_set_option($socket, SOL_SOCKET, SO_RCVTIMEO, ['sec' => 1, 'usec' => 0]);

        if (! @socket_bind($socket, $host, $port)) {
            $message = socket_strerror(socket_last_error($socket));
            socket_close($socket);
            throw new RuntimeException(sprintf('SNMP trap listener bind hatasi: %s:%d => %s', $host, $port, $message));
        }

        Log::info('SNMP trap listener basladi.', [
            'host' => $host,
            'port' => $port,
            'max_packets' => $maxPackets,
        ]);

        $processed = 0;

        try {
            while ($maxPackets === null || $processed < $maxPackets) {
                $remoteIp = null;
                $remotePort = null;
                $packet = '';
                $bytes = @socket_recvfrom($socket, $packet, $bufferBytes, 0, $remoteIp, $remotePort);

                if ($bytes === false) {
                    $error = socket_last_error($socket);
                    if (in_array($error, [11, 35, 10035], true)) {
                        continue;
                    }

                    throw new RuntimeException('SNMP trap socket okuma hatasi: '.socket_strerror($error));
                }

                if ($bytes <= 0) {
                    continue;
                }

                $processed++;
                $result = $this->ingestPacket($packet, $remoteIp, (int) $remotePort);
                if ($onPacket) {
                    $onPacket($result);
                }
            }
        } finally {
            socket_close($socket);
        }

        return $processed;
    }

    public function ingestPacket(string $packet, ?string $remoteIp = null, ?int $remotePort = null): array
    {
        try {
            $decoded = $this->packetDecoder->decode($packet);
            $payload = $this->buildIngestPayload($decoded, $remoteIp, $remotePort);
            $result = $this->snmpTrapIngestService->ingest($payload, $remoteIp);

            Log::info('SNMP trap paketi islendi.', [
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'trap_type' => $payload['trap_type'] ?? null,
                'if_index' => $payload['if_index'] ?? null,
                'switch_id' => $result['switch_id'] ?? null,
            ]);

            return [
                'ok' => true,
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'decoded' => $decoded,
                'result' => $result,
            ];
        } catch (ValidationException $exception) {
            $errors = $exception->errors();
            $message = collect($errors)->flatten()->first() ?: 'Trap dogrulama hatasi.';
            Log::warning('SNMP trap dogrulama hatasi.', [
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'errors' => $errors,
            ]);

            return [
                'ok' => false,
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'error' => $message,
                'errors' => $errors,
            ];
        } catch (\Throwable $exception) {
            Log::warning('SNMP trap islenemedi.', [
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'error' => $exception->getMessage(),
            ]);

            return [
                'ok' => false,
                'remote_ip' => $remoteIp,
                'remote_port' => $remotePort,
                'error' => $exception->getMessage(),
            ];
        }
    }

    protected function buildIngestPayload(array $decoded, ?string $remoteIp, ?int $remotePort): array
    {
        $switchIp = $decoded['agent_address'] ?: $remoteIp;
        $ifIndex = (int) ($decoded['if_index'] ?? 0);
        if ($ifIndex <= 0) {
            throw ValidationException::withMessages([
                'if_index' => 'Trap paketinden if_index cikartilamadi.',
            ]);
        }

        if ((bool) config('services.nac.trap_validate_community', false)) {
            $this->validateCommunityMatch($switchIp, (string) ($decoded['community'] ?? ''));
        }

        return [
            'switch_ip' => $switchIp,
            'source_ip' => $remoteIp,
            'if_index' => $ifIndex,
            'if_name' => $decoded['if_name'] ?? null,
            'if_descr' => $decoded['if_descr'] ?? null,
            'admin_status' => $decoded['admin_status'] ?? null,
            'oper_status' => $decoded['oper_status'] ?? null,
            'speed' => $decoded['speed'] ?? null,
            'occurred_at' => now()->toIso8601String(),
            'trap_oid' => $decoded['trap_oid'] ?? null,
            'trap_type' => $decoded['trap_type'] ?? null,
            'varbinds' => $decoded['varbinds'] ?? [],
            'snmp_version' => $decoded['snmp_version'] ?? null,
            'community' => $decoded['community'] ?? null,
            'source_port' => $remotePort,
        ];
    }

    protected function validateCommunityMatch(?string $switchIp, string $community): void
    {
        if (! $switchIp || $community === '') {
            return;
        }

        $switch = NetworkSwitch::query()->where('ip_address', $switchIp)->first();
        if (! $switch || ! $switch->snmp_community) {
            return;
        }

        if (! hash_equals((string) $switch->snmp_community, $community)) {
            throw ValidationException::withMessages([
                'community' => 'Trap community bilgisi switch kaydi ile eslesmedi.',
            ]);
        }
    }
}
