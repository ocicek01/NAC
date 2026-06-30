<?php

namespace App\Services;

use App\Models\NetworkSwitch;
use Illuminate\Support\Str;

class HpSnmpVendorEnricher implements SnmpVendorEnricher
{
    public function supports(NetworkSwitch $switch): bool
    {
        $vendor = strtolower($switch->vendor);

        return Str::contains($vendor, 'hp') || Str::contains($vendor, 'hpe');
    }

    public function enrich(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        return $portsByIfIndex;
    }
}
