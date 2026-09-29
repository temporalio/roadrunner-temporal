<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Temporal\Tests\Activity\SimpleActivity;
use Temporal\Tests\Workflow\SimpleWorkflow;
use Temporal\Worker\WorkerOptions;

$factory = \Temporal\WorkerFactory::create();

$worker = $factory->newWorker('default', WorkerOptions::new()->withWorkerStopTimeout(10));

$worker->registerWorkflowTypes(SimpleWorkflow::class);
$worker->registerActivityImplementations(new SimpleActivity());

$factory->run();
