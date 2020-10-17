<?php

use Temporal\Client;

// worker.php
ini_set('display_errors', 'stderr');
include "vendor/autoload.php";

$relay = new Spiral\Goridge\StreamRelay(STDIN, STDOUT);
$w = new Spiral\RoadRunner\Worker($relay);

$temporal = new Client\WorkerFactory($w);

$temporal
    ->createActivityWorker("default", [Client\ActivityWorker::MAX_CONCURRENT_ACTIVITY_TASK_POLLERS => 2])
    ->registerActivity('hello', function (string $name): string {
        return sprintf("Hello, %s!", $name);
    });

$temporal->run();
