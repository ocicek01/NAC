<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('switch_ports', function (Blueprint $table) {
            $table->string('if_name')->nullable()->after('if_index');
            $table->string('if_descr')->nullable()->after('if_name');
            $table->timestamp('last_seen')->nullable()->after('duplex');
            $table->timestamp('last_change')->nullable()->after('last_seen');
            $table->string('status_source')->default('snmp_poll')->after('last_change');
            $table->json('raw_status')->nullable()->after('status_source');
        });

        Schema::table('switches', function (Blueprint $table) {
            $table->timestamp('last_polled_at')->nullable()->after('snmp_retries');
            $table->unsignedInteger('consecutive_polling_failures')->default(0)->after('last_polled_at');
            $table->text('polling_error')->nullable()->after('consecutive_polling_failures');
        });

        Schema::table('switch_ports', function (Blueprint $table) {
            $table->dropUnique('switch_ports_switch_id_port_index_unique');
            $table->unique(['switch_id', 'port_index']);
            $table->unique(['switch_id', 'if_index']);
        });
    }

    public function down(): void
    {
        Schema::table('switch_ports', function (Blueprint $table) {
            $table->dropUnique(['switch_id', 'if_index']);
            $table->dropUnique(['switch_id', 'port_index']);
            $table->unique(['switch_id', 'port_index']);
        });

        Schema::table('switches', function (Blueprint $table) {
            $table->dropColumn([
                'last_polled_at',
                'consecutive_polling_failures',
                'polling_error',
            ]);
        });

        Schema::table('switch_ports', function (Blueprint $table) {
            $table->dropColumn([
                'if_name',
                'if_descr',
                'last_seen',
                'last_change',
                'status_source',
                'raw_status',
            ]);
        });
    }
};
