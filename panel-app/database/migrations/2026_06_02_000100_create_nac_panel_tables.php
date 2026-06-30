<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::create('zones', function (Blueprint $table) {
            $table->id();
            $table->string('name');
            $table->string('slug')->unique();
            $table->text('description')->nullable();
            $table->string('status')->default('normal');
            $table->timestamps();
        });

        Schema::create('switches', function (Blueprint $table) {
            $table->id();
            $table->foreignId('zone_id')->constrained('zones')->cascadeOnDelete();
            $table->string('hostname')->unique();
            $table->string('ip_address')->unique();
            $table->string('vendor');
            $table->string('model');
            $table->string('location')->nullable();
            $table->string('status')->default('online');
            $table->boolean('managed')->default(true);
            $table->string('nac_mode')->default('monitor');
            $table->unsignedInteger('port_count')->default(24);
            $table->timestamp('last_seen_at')->nullable();
            $table->timestamps();
        });

        Schema::create('switch_ports', function (Blueprint $table) {
            $table->id();
            $table->foreignId('switch_id')->constrained('switches')->cascadeOnDelete();
            $table->unsignedInteger('port_index');
            $table->string('port_name');
            $table->string('status')->default('down');
            $table->string('port_type')->default('access');
            $table->string('nac_mode')->default('inherit');
            $table->unsignedInteger('vlan_id')->nullable();
            $table->string('speed')->nullable();
            $table->string('duplex')->default('Full');
            $table->boolean('poe_enabled')->default(false);
            $table->decimal('poe_power', 8, 2)->default(0);
            $table->timestamp('last_change_at')->nullable();
            $table->timestamps();
            $table->unique(['switch_id', 'port_index']);
        });

        Schema::create('endpoints', function (Blueprint $table) {
            $table->id();
            $table->string('mac_address')->unique();
            $table->string('ip_address')->nullable();
            $table->string('hostname')->nullable();
            $table->string('user_name')->nullable();
            $table->string('device_type')->nullable();
            $table->string('policy_name')->nullable();
            $table->string('role_name')->nullable();
            $table->unsignedInteger('vlan_id')->nullable();
            $table->string('status')->default('authenticated');
            $table->timestamp('last_seen_at')->nullable();
            $table->timestamps();
        });

        Schema::create('endpoint_locations', function (Blueprint $table) {
            $table->id();
            $table->foreignId('endpoint_id')->constrained('endpoints')->cascadeOnDelete();
            $table->foreignId('switch_id')->constrained('switches')->cascadeOnDelete();
            $table->foreignId('switch_port_id')->constrained('switch_ports')->cascadeOnDelete();
            $table->unsignedInteger('vlan_id')->nullable();
            $table->timestamp('first_seen_at')->nullable();
            $table->timestamp('last_seen_at')->nullable();
            $table->timestamps();
        });

        Schema::create('nac_audit_logs', function (Blueprint $table) {
            $table->id();
            $table->foreignId('actor_id')->nullable()->constrained('users')->nullOnDelete();
            $table->string('action');
            $table->string('target_type');
            $table->unsignedBigInteger('target_id');
            $table->foreignId('switch_id')->nullable()->constrained('switches')->nullOnDelete();
            $table->foreignId('switch_port_id')->nullable()->constrained('switch_ports')->nullOnDelete();
            $table->foreignId('endpoint_id')->nullable()->constrained('endpoints')->nullOnDelete();
            $table->json('old_value')->nullable();
            $table->json('new_value')->nullable();
            $table->string('ip_address')->nullable();
            $table->timestamp('created_at')->useCurrent();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('nac_audit_logs');
        Schema::dropIfExists('endpoint_locations');
        Schema::dropIfExists('endpoints');
        Schema::dropIfExists('switch_ports');
        Schema::dropIfExists('switches');
        Schema::dropIfExists('zones');
    }
};
