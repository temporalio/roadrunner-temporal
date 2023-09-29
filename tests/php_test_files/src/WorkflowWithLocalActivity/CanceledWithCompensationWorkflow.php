<?php

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class CanceledWithCompensationWorkflow
{
    private array $status = [];

    #[Workflow\QueryMethod(name: 'getStatus')]
    public function getStatus(): array
    {
        return $this->status;
    }

    #[WorkflowMethod(name: 'CanceledWithCompensationWorkflow')]
    public function handler()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()->withStartToCloseTimeout(5)
        );

        // waits for 2 seconds
        $slow = $simple->slow('DOING SLOW ACTIVITY');

        try {
            $this->status[] = 'yield';
            $result = yield $slow;
        } catch (CanceledFailure $e) {
            $this->status[] = 'rollback';

            try {
                // must fail again
                $result = yield $slow;
            } catch (CanceledFailure $e) {
                $this->status[] = 'captured retry';
            }

            try {
                // fail since on canceled context
                $result = yield $simple->echo('echo must fail');
            } catch (CanceledFailure $e) {
                $this->status[] = 'captured promise on canceled';
            }

            $scope = Workflow::asyncDetached(
                function () use ($simple) {
                    $this->status[] = 'START rollback';

                    $second = yield $simple->echo('rollback');

                    $this->status[] = sprintf("RESULT (%s)", $second);

                    if ($second !== 'ROLLBACK') {
                        $this->status[] = 'FAIL rollback';
                        return 'failed to compensate ' . $second;
                    }
                    $this->status[] = 'DONE rollback';

                    return 'OK';
                }
            );

            $this->status[] = 'WAIT ROLLBACK';
            $result = yield $scope;
            $this->status[] = 'COMPLETE rollback';
        }

        $this->status[] = 'result: ' . $result;
        return $result;
    }
}
