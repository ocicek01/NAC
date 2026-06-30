<?php

use App\Http\Controllers\Api\NacActionController;
use App\Http\Controllers\Api\DiscoveryJobController;
use App\Http\Controllers\Api\SwitchController;
use App\Http\Controllers\Api\SwitchPortController;
use App\Http\Controllers\Api\ZoneController;
use Illuminate\Support\Facades\Route;

Route::get('/zones', [ZoneController::class, 'index']);
Route::get('/zones/{zone}', [ZoneController::class, 'show']);

Route::get('/switches', [SwitchController::class, 'index']);
Route::post('/switches', [SwitchController::class, 'store']);
Route::get('/switches/{switch}', [SwitchController::class, 'show']);
Route::put('/switches/{switch}', [SwitchController::class, 'update']);
Route::put('/switches/{switch}/nac-mode', [SwitchController::class, 'updateNacMode']);
Route::post('/switches/{switch}/rediscover-ports', [SwitchController::class, 'rediscoverPorts']);
Route::get('/switches/{switch}/ports', [SwitchController::class, 'ports']);
Route::get('/discovery-jobs/{job}', [DiscoveryJobController::class, 'show']);

Route::get('/switch-ports/{port}', [SwitchPortController::class, 'show']);
Route::get('/switch-ports/{port}/lldp', [SwitchPortController::class, 'lldp']);
Route::post('/switch-ports/{port}/rediscover', [SwitchPortController::class, 'rediscover']);
Route::put('/switch-ports/{port}/nac-mode', [SwitchPortController::class, 'updateNacMode']);
Route::post('/switch-ports/{port}/actions', [NacActionController::class, 'store']);
