<?php

namespace Temporal\Tests\Workflow;

use Temporal\Exception\Failure\CanceledFailure;
use Temporal\Workflow;
use Temporal\Workflow\WorkflowMethod;

#[Workflow\WorkflowInterface]
class CanceledNestedWorkflow
{
    private array $status = [];

    #[Workflow\QueryMethod(name: 'getStatus')]
    public function getStatus(): array
    {
        return $this->status;
    }

    #[WorkflowMethod(name: 'CanceledNestedWorkflow')]
    public function handler()
    {
        $this->status[] = 'begin';
        try {
            yield Workflow::async(
                function () {
                    $this->status[] = 'first scope';

                    $scope = Workflow::async(
                        function () {
                            $this->status[] = 'second scope';

                            try {
                                yield Workflow::timer(20);
                            } catch (CanceledFailure $e) {
                                $this->status[] = 'second scope canceled';
                                throw $e;
                            }

                            $this->status[] = 'second scope done';
                        }
                    )->onCancel(
                        function () {
                            $this->status[] = 'close second scope';
                        }
                    );

                    try {
                        yield Workflow::timer(20);
                    } catch (CanceledFailure $e) {
                        $this->status[] = 'first scope canceled';
                        throw $e;
                    }

                    $this->status[] = 'first scope done';

                    yield $scope;
                }
            )->onCancel(
                function () {
                    $this->status[] = 'close first scope';
                }
            );
        } catch (CanceledFailure $e) {
            $this->status[] = 'close process';

            return 'CANCELED';
        }

        return 'OK';
    }
}
