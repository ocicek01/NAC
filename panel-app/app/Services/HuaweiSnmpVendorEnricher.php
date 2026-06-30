<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use Illuminate\Support\Str;

class HuaweiSnmpVendorEnricher implements SnmpVendorEnricher
{
    public function supports(NetworkSwitch $switch): bool
    {
        return Str::contains(strtolower($switch->vendor), 'huawei');
    }

    public function enrich(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        return $portsByIfIndex;
    }
}
