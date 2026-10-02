<?php

declare(strict_types=1);

require __DIR__ . '/vendor/autoload.php';

use Temporal\Activity\ActivityInterface;
use Temporal\Activity\ActivityMethod;
use Temporal\WorkerFactory;
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

$factory = WorkerFactory::create();
$factory->newWorker('default')
    ->registerWorkflowTypes(LambdaProbeWorkflow::class)
    ->registerActivityImplementations(new LambdaProbeActivity());

$factory->run();
