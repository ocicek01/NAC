<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('switches', function (Blueprint $table) {
            $table->string('snmp_version')->nullable()->after('port_count');
            $table->string('snmp_community')->nullable()->after('snmp_version');
            $table->unsignedInteger('snmp_port')->nullable()->after('snmp_community');
            $table->unsignedInteger('snmp_timeout_ms')->nullable()->after('snmp_port');
            $table->unsignedInteger('snmp_retries')->nullable()->after('snmp_timeout_ms');
        });
    }

    public function down(): void
    {
        Schema::table('switches', function (Blueprint $table) {
            $table->dropColumn([
                'snmp_version',
                'snmp_community',
                'snmp_port',
                'snmp_timeout_ms',
                'snmp_retries',
            ]);
        });
    }
};
