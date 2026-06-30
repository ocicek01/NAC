<?php

namespace App\Services;

use App\Models\NetworkSwitch;

class NullSnmpVendorEnricher implements SnmpVendorEnricher
{
    public function supports(NetworkSwitch $switch): bool
    {
        return true;
    }

    public function enrich(NetworkSwitch $switch, array $portsByIfIndex): array
    {
        return $portsByIfIndex;
    }
}
