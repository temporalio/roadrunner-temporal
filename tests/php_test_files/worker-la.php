<?php

declare(strict_types=1);

require __DIR__ . '/../vendor/autoload.php';

use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Tests\Workflow\CancelSignaledChildWorkflow;
use Temporal\Tests\Workflow\ChildStubWorkflow;
use Temporal\Tests\Workflow\EmptyWorkflow;
use Temporal\Tests\Workflow\QueryWorkflow;
use Temporal\Tests\Workflow\RuntimeSignalWorkflow;
use Temporal\Tests\Workflow\SignalChildViaStubWorkflow;
use Temporal\Tests\Workflow\SimpleSignaledWorkflow;
use Temporal\Tests\Workflow\SimpleSignaledWorkflowWithSleep;
use Temporal\Tests\Workflow\WithChildStubWorkflow;
use Temporal\Tests\Workflow\WithChildWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ActivityStubWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\AsyncActivityWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\BinaryWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledMidflightWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledNestedWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledScopeWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledSingleScopeWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledWithCompensationWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\CanceledWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ChainedWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ContinuableWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ExceptionalActivityWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\LoopWithSignalCoroutinesWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\LoopWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ParallelScopesWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\ProtoPayloadWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\SagaWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\SideEffectWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\SimpleDTOWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\SimpleWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\TimerWorkflow;
use Temporal\Tests\WorkflowWithLocalActivity\WorkflowWithSequence;
use Temporal\Tests\WorkflowWithLocalActivity\WorkflowWithSignaledSteps;

/**
 * @param string $dir
 * @return array<string>
 */
$getClasses = static function (string $dir): iterable {
    $files = glob($dir . '/*.php');

    foreach ($files as $file) {
        yield substr(basename($file), 0, -4);
    }
};

$factory = \Temporal\WorkerFactory::create();

$worker = $factory->newWorker('default');

$worker->registerWorkflowTypes(ActivityStubWorkflow::class);
$worker->registerWorkflowTypes(AsyncActivityWorkflow::class);
$worker->registerWorkflowTypes(BinaryWorkflow::class);
$worker->registerWorkflowTypes(CanceledMidflightWorkflow::class);
$worker->registerWorkflowTypes(CanceledNestedWorkflow::class);
$worker->registerWorkflowTypes(CanceledScopeWorkflow::class);
$worker->registerWorkflowTypes(CanceledSingleScopeWorkflow::class);
$worker->registerWorkflowTypes(CanceledWithCompensationWorkflow::class);
$worker->registerWorkflowTypes(CanceledWorkflow::class);
$worker->registerWorkflowTypes(ContinuableWorkflow::class);
$worker->registerWorkflowTypes(ChainedWorkflow::class);
$worker->registerWorkflowTypes(ExceptionalActivityWorkflow::class);
$worker->registerWorkflowTypes(LoopWithSignalCoroutinesWorkflow::class);
$worker->registerWorkflowTypes(LoopWorkflow::class);
$worker->registerWorkflowTypes(ParallelScopesWorkflow::class);
$worker->registerWorkflowTypes(ProtoPayloadWorkflow::class);
$worker->registerWorkflowTypes(SagaWorkflow::class);
$worker->registerWorkflowTypes(SideEffectWorkflow::class);
$worker->registerWorkflowTypes(SimpleDTOWorkflow::class);
$worker->registerWorkflowTypes(SimpleWorkflow::class);
$worker->registerWorkflowTypes(TimerWorkflow::class);
$worker->registerWorkflowTypes(WorkflowWithSequence::class);
$worker->registerWorkflowTypes(WorkflowWithSignaledSteps::class);
$worker->registerWorkflowTypes(EmptyWorkflow::class);
$worker->registerWorkflowTypes(SimpleSignaledWorkflow::class);
$worker->registerWorkflowTypes(CancelSignaledChildWorkflow::class);
$worker->registerWorkflowTypes(WithChildWorkflow::class);
$worker->registerWorkflowTypes(WithChildStubWorkflow::class);
$worker->registerWorkflowTypes(ChildStubWorkflow::class);
$worker->registerWorkflowTypes(SignalChildViaStubWorkflow::class);
$worker->registerWorkflowTypes(QueryWorkflow::class);
$worker->registerWorkflowTypes(SimpleSignaledWorkflowWithSleep::class);
$worker->registerWorkflowTypes(RuntimeSignalWorkflow::class);

$worker->registerActivity(SimpleLocalActivity::class);

$factory->run();
