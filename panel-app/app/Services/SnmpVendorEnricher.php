<?php

namespace App\Services;

use App\Models\NetworkSwitch;

interface SnmpVendorEnricher
{
    public function supports(NetworkSwitch $switch): bool;

    /**
     * @param array<int, array<string, mixed>> $portsByIfIndex
     * @return array<int, array<string, mixed>>
     */
    public function enrich(NetworkSwitch $switch, array $portsByIfIndex): array;
}
