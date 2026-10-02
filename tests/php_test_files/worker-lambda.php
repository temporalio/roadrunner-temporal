<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\Activity\ActivityOptions;
use Temporal\Common\RetryOptions;
use Temporal\WorkerFactory;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowInterface;
use Temporal\Workflow\WorkflowMethod;

$boots = \getenv('LAMBDA_BOOT_LOG');
if ($boots !== false && $boots !== '') {
    \file_put_contents($boots, \getmypid() . "\n", \FILE_APPEND);
}

#[ActivityInterface(prefix: 'LambdaProbe.')]
class LambdaProbeActivity
{
    #[ActivityMethod]
    public function noop(): void {}

    #[ActivityMethod]
    public function slow(int $seconds): string
    {
        \sleep($seconds);

        return 'drained';
    }
}

#[WorkflowInterface]
class LambdaProbeWorkflow
{
    #[WorkflowMethod(name: 'LambdaProbeWorkflow')]
    public function handler()
    {
        return 'ok';
    }
}

#[WorkflowInterface]
class LambdaSlowWorkflow
{
    #[WorkflowMethod(name: 'LambdaSlowWorkflow')]
    public function handler(int $seconds)
    {
        return yield Workflow::newActivityStub(
            LambdaProbeActivity::class,
            ActivityOptions::new()
                ->withStartToCloseTimeout('60 seconds')
                ->withRetryOptions(RetryOptions::new()->withMaximumAttempts(1)),
        )->slow($seconds);
    }
}

$factory = WorkerFactory::create();
$factory->newWorker('default')
    ->registerWorkflowTypes(LambdaProbeWorkflow::class, LambdaSlowWorkflow::class)
    ->registerActivityImplementations(new LambdaProbeActivity());

$factory->run();
