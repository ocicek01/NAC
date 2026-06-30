<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('switch_ports', function (Blueprint $table) {
            $table->unsignedInteger('if_index')->nullable()->after('switch_id');
            $table->string('port_description')->nullable()->after('port_name');
            $table->string('admin_status')->nullable()->after('status');
            $table->string('oper_status')->nullable()->after('admin_status');
            $table->unsignedInteger('native_vlan')->nullable()->after('vlan_id');
            $table->json('allowed_vlans')->nullable()->after('native_vlan');
            $table->unsignedInteger('mac_count')->default(0)->after('allowed_vlans');
            $table->timestamp('last_discovered_at')->nullable()->after('last_change_at');
        });
    }

    public function down(): void
    {
        Schema::table('switch_ports', function (Blueprint $table) {
            $table->dropColumn([
                'if_index',
                'port_description',
                'admin_status',
                'oper_status',
                'native_vlan',
                'allowed_vlans',
                'mac_count',
                'last_discovered_at',
            ]);
        });
    }
};
