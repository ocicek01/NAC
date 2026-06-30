<?php

use App\Http\Controllers\Web\DevicePageController;
use App\Http\Controllers\Web\SwitchPageController;
use Illuminate\Support\Facades\Route;

Route::get('/', [SwitchPageController::class, 'index']);
Route::get('/switches', [SwitchPageController::class, 'index'])->name('switches.index');
Route::get('/devices', [DevicePageController::class, 'index'])->name('devices.index');
Route::post('/devices/{mac}/approve', [DevicePageController::class, 'approve'])->name('devices.approve');
Route::post('/devices/{mac}/guest', [DevicePageController::class, 'guest'])->name('devices.guest');
Route::post('/devices/{mac}/block', [DevicePageController::class, 'block'])->name('devices.block');
Route::post('/devices/{mac}/retire', [DevicePageController::class, 'retire'])->name('devices.retire');
Route::get('/switches/create', [SwitchPageController::class, 'create'])->name('switches.create');
Route::post('/switches', [SwitchPageController::class, 'store'])->name('switches.store');
Route::get('/switches/{zone:slug}', [SwitchPageController::class, 'zone'])->name('switches.zone');
Route::get('/switches/{zone:slug}/{switch}', [SwitchPageController::class, 'show'])->name('switches.show');
