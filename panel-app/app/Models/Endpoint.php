<?php

namespace App\Models;

use Illuminate\Database\Eloquent\Factories\HasFactory;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Database\Eloquent\Relations\HasMany;

class Endpoint extends Model
{
    use HasFactory;

    protected $fillable = [
        'mac_address',
        'ip_address',
        'hostname',
        'user_name',
        'device_type',
        'policy_name',
        'role_name',
        'vlan_id',
        'status',
        'last_seen_at',
    ];

    protected $casts = [
        'last_seen_at' => 'datetime',
    ];

    public function locations(): HasMany
    {
        return $this->hasMany(EndpointLocation::class);
    }
}
