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
    ->addWorkflow(Tests\Workflow\SideEffectWorkflow::class)
    ->addWorkflow(Tests\Workflow\SimpleSignalledWorkflowWithSleep::class)
    ->addWorkflow(Tests\Workflow\QueryWorkflow::class)
    ->addWorkflow(Tests\Workflow\EmptyWorkflow::class)
    ->addWorkflow(Tests\Workflow\RuntimeSignalWorkflow::class)
    ->addWorkflow(Tests\Workflow\CancelledScopeWorkflow::class)
    ->addWorkflow(Tests\Workflow\WorkflowWithSignalledSteps::class)
    ->addWorkflow(Tests\Workflow\WorkflowWithSequence::class)
    ->addWorkflow(Tests\Workflow\ChainedWorkflow::class)
    ->addWorkflow(Tests\Workflow\WithChildWorkflow::class)
    ->addWorkflow(Tests\Workflow\WithChildStubWorkflow::class)
    ->addWorkflow(Tests\Workflow\SimpleHeartbeatWorkflow::class)
    ->addActivity(Tests\Activity\SimpleActivity::class)
    ->addActivity(Tests\Activity\HeartBeatActivity::class);

$worker->run();
