<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\BelongsTo;
use Illuminate\Database\Eloquent\Relations\HasMany;
use Illuminate\Database\Eloquent\Relations\HasOne;

class SwitchPort extends Model
{
    use HasFactory;

    protected $fillable = [
        'switch_id',
        'if_index',
        'port_index',
        'port_name',
        'port_description',
        'status',
        'admin_status',
        'oper_status',
        'port_type',
        'nac_mode',
        'vlan_id',
        'native_vlan',
        'allowed_vlans',
        'mac_count',
        'speed',
        'duplex',
        'poe_enabled',
        'poe_power',
        'last_change_at',
        'last_discovered_at',
    ];

    protected $casts = [
        'if_index' => 'integer',
        'vlan_id' => 'integer',
        'native_vlan' => 'integer',
        'mac_count' => 'integer',
        'allowed_vlans' => 'array',
        'poe_enabled' => 'boolean',
        'poe_power' => 'decimal:2',
        'last_change_at' => 'datetime',
        'last_discovered_at' => 'datetime',
    ];

    public function switch(): BelongsTo
    {
        return $this->belongsTo(NetworkSwitch::class, 'switch_id');
    }

    public function endpointLocations(): HasMany
    {
        return $this->hasMany(EndpointLocation::class, 'switch_port_id');
    }

    public function currentLocation(): HasOne
    {
        return $this->hasOne(EndpointLocation::class, 'switch_port_id')->latestOfMany('last_seen_at');
    }
}
