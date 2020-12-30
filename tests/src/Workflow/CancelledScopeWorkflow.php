<?php

namespace Temporal\Tests\Workflow;

use Temporal\Client\Activity\ActivityOptions;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleActivity;

class CancelledScopeWorkflow
{
    #[WorkflowMethod(name: 'CancelledScopeWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleActivity::class,
            ActivityOptions::new()->withStartToCloseTimeout(5)
        );

        $cancelled = 'not';

        $scope = Workflow::newCancellationScope(function () use ($simple) {
            yield Workflow::timer(2);
            yield $simple->echo('hell o');
        })->onCancel(function () use (&$cancelled) {
            $cancelled = 'yes';
        });

        yield Workflow::timer(1);
        $scope->cancel();

        return $cancelled;
    }
}