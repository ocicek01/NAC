<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;

class NetworkSwitch extends Model
{
    use HasFactory;

    protected $table = 'switches';

    protected $fillable = [
        'zone_id',
        'hostname',
        'ip_address',
        'vendor',
        'model',
        'location',
        'status',
        'managed',
        'nac_mode',
        'port_count',
        'snmp_version',
        'snmp_community',
        'snmp_port',
        'snmp_timeout_ms',
        'snmp_retries',
        'last_polled_at',
        'consecutive_polling_failures',
        'polling_error',
        'last_seen_at',
    ];

    protected $casts = [
        'managed' => 'boolean',
        'snmp_port' => 'integer',
        'snmp_timeout_ms' => 'integer',
        'snmp_retries' => 'integer',
        'last_polled_at' => 'datetime',
        'consecutive_polling_failures' => 'integer',
        'last_seen_at' => 'datetime',
    ];

    public function zone(): BelongsTo
    {
        return $this->belongsTo(Zone::class);
    }

    public function ports(): HasMany
    {
        return $this->hasMany(SwitchPort::class, 'switch_id')->orderBy('port_index');
    }

    public function auditLogs(): HasMany
    {
        return $this->hasMany(NacAuditLog::class, 'switch_id');
    }
}

