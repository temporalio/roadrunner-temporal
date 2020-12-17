<?php

declare(strict_types=1);

use Temporal\Client\Worker;
use Temporal\Client\Worker\Transport\RoadRunner;
use Temporal\Tests;

require __DIR__ . '/vendor/autoload.php';

$worker = new Worker(RoadRunner::pipes());

$worker->createAndRegister()
    ->addWorkflow(Tests\Workflow\SimpleWorkflow::class)
    ->addWorkflow(Tests\Workflow\SimpleSignalledWorkflow::class)
    ->addWorkflow(Tests\Workflow\ParallelScopesWorkflow::class)
    ->addWorkflow(Tests\Workflow\TimerWorkflow::class)
    ->addActivity(Tests\Activity\SimpleActivity::class);

$worker->run();
