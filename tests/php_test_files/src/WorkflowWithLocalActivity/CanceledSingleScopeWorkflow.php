<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;
use Temporal\Tests\Activity\SimpleLocalActivity;

#[Workflow\WorkflowInterface]
class CanceledSingleScopeWorkflow
{
    private array $status = [];

    #[Workflow\QueryMethod(name: 'getStatus')]
    public function getStatus(): array
    {
        return $this->status;
    }

    #[WorkflowMethod(name: 'CanceledSingleScopeWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()
                ->withStartToCloseTimeout(5)
        );

        $this->status[] = 'start';
        try {
            yield Workflow::async(
                function () use ($simple) {
                    try {
                        $this->status[] = 'in scope';
                        yield $simple->slow('1');
                    } catch (CanceledFailure $e) {
                        // after process is complete, do not use for business logic
                        $this->status[] = 'captured in scope';
                        throw $e;
                    }
                }
            )->onCancel(
                function () {
                    $this->status[] = 'on cancel';
                }
            );
        } catch (CanceledFailure $e) {
            $this->status[] = 'captured in process';
        }

        return 'OK';
    }
}
