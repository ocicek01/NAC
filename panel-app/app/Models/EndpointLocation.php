<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;

class EndpointLocation extends Model
{
    use HasFactory;

    protected $fillable = [
        'endpoint_id',
        'switch_id',
        'switch_port_id',
        'vlan_id',
        'first_seen_at',
        'last_seen_at',
    ];

    protected $casts = [
        'first_seen_at' => 'datetime',
        'last_seen_at' => 'datetime',
    ];

    public function endpoint(): BelongsTo
    {
        return $this->belongsTo(Endpoint::class);
    }

    public function switch(): BelongsTo
    {
        return $this->belongsTo(NetworkSwitch::class, 'switch_id');
    }

    public function switchPort(): BelongsTo
    {
        return $this->belongsTo(SwitchPort::class, 'switch_port_id');
    }
}
