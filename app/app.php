<?php

// worker.php
ini_set('display_errors', 'stderr');
include "vendor/autoload.php";

$relay = new Spiral\Goridge\StreamRelay(STDIN, STDOUT);
$w = new Spiral\RoadRunner\Worker($relay);

while ($payload = $w->receive($header)) {
    try {
        $w->send($payload, $header);
    } catch (\Throwable $e) {
        $w->error((string) $e);
    }
}
