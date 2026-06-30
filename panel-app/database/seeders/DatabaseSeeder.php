<?php

namespace Database\Seeders;

use App\Models\User;
use App\Models\Zone;
use Illuminate\Database\Console\Seeds\WithoutModelEvents;
use Illuminate\Database\Seeder;
use Illuminate\Support\Str;

class DatabaseSeeder extends Seeder
{
    use WithoutModelEvents;

    public function run(): void
    {
        User::factory()->create([
            'name' => 'Test User',
            'email' => 'test@example.com',
        ]);

        Zone::firstOrCreate(
            ['slug' => Str::slug('Kutuphane')],
            [
                'name' => 'Kutuphane',
                'description' => null,
                'status' => 'normal',
            ]
        );
    }
}
