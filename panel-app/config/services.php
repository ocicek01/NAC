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
        'default_allow_vlan' => env('NAC_DEFAULT_ALLOW_VLAN', 106),
        'guest_vlan' => env('NAC_GUEST_VLAN', 300),
        'quarantine_vlan' => env('NAC_QUARANTINE_VLAN', 333),
    ],

];

