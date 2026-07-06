<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Third Party Services
    |--------------------------------------------------------------------------
    |
    | This file is for storing the credentials for third party services such
    | as Mailgun, Postmark, AWS and more. This file provides the de facto
    | location for this type of information, allowing packages to have
    | a conventional file to locate the various service credentials.
    |
    */

    'postmark' => [
        'key' => env('POSTMARK_API_KEY'),
    ],

    'resend' => [
        'key' => env('RESEND_API_KEY'),
    ],

    'ses' => [
        'key' => env('AWS_ACCESS_KEY_ID'),
        'secret' => env('AWS_SECRET_ACCESS_KEY'),
        'region' => env('AWS_DEFAULT_REGION', 'us-east-1'),
    ],

    'slack' => [
        'notifications' => [
            'bot_user_oauth_token' => env('SLACK_BOT_USER_OAUTH_TOKEN'),
            'channel' => env('SLACK_BOT_USER_DEFAULT_CHANNEL'),
        ],
    ],

    'nac' => [
        'base_url' => env('NAC_API_URL', 'http://127.0.0.1:8080'),
        'timeout' => env('NAC_API_TIMEOUT', 10),
        'long_timeout' => env('NAC_API_LONG_TIMEOUT', 120),
        'connect_timeout' => env('NAC_API_CONNECT_TIMEOUT', 3),
        'retry_times' => env('NAC_API_RETRY_TIMES', 2),
        'retry_sleep_ms' => env('NAC_API_RETRY_SLEEP_MS', 200),
        'switch_detail_remote_enrichment' => env('NAC_SWITCH_DETAIL_REMOTE_ENRICHMENT', false),
        'discovery_schedule_enabled' => env('NAC_DISCOVERY_SCHEDULE_ENABLED', true),
        'discovery_schedule_minutes' => env('NAC_DISCOVERY_SCHEDULE_MINUTES', 10),
        'trap_ingest_enabled' => env('NAC_TRAP_INGEST_ENABLED', true),
        'trap_ingest_token' => env('NAC_TRAP_INGEST_TOKEN'),
        'trap_listener_enabled' => env('NAC_TRAP_LISTENER_ENABLED', true),
        'trap_listener_host' => env('NAC_TRAP_LISTENER_HOST', '0.0.0.0'),
        'trap_listener_port' => env('NAC_TRAP_LISTENER_PORT', 162),
        'trap_listener_buffer_bytes' => env('NAC_TRAP_LISTENER_BUFFER_BYTES', 65535),
        'trap_validate_community' => env('NAC_TRAP_VALIDATE_COMMUNITY', false),
        'default_allow_vlan' => env('NAC_DEFAULT_ALLOW_VLAN', 106),
        'guest_vlan' => env('NAC_GUEST_VLAN', 300),
        'quarantine_vlan' => env('NAC_QUARANTINE_VLAN', 333),
    ],

];

