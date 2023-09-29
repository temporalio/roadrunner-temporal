<?php

/**
 * This file is part of Temporal package.
 *
 * For the full copyright and license information, please view the LICENSE
 * file that was distributed with this source code.
 */

namespace Temporal\Tests\WorkflowWithLocalActivity;

use Temporal\Activity\LocalActivityOptions;
use Temporal\Common\RetryOptions;
use Temporal\Tests\Activity\SimpleLocalActivity;
use Temporal\Workflow;

#[Workflow\WorkflowInterface]
class SagaWorkflow
{
    #[Workflow\WorkflowMethod(name: 'SagaWorkflow')]
    public function run()
    {
        $simple = Workflow::newActivityStub(
            SimpleLocalActivity::class,
            LocalActivityOptions::new()
                ->withScheduleToCloseTimeout(60)
                ->withStartToCloseTimeout(60)
                ->withRetryOptions(RetryOptions::new()->withMaximumAttempts(1))
        );

        $saga = new Workflow\Saga();
        $saga->setParallelCompensation(true);

        try {
            yield $simple->echo('test');
            $saga->addCompensation(
                function () use ($simple) {
                    yield $simple->echo('compensate echo');
                }
            );

            yield $simple->lower('TEST');
            $saga->addCompensation(
                function () use ($simple) {
                    yield $simple->lower('COMPENSATE LOWER');
                }
            );

            yield $simple->fail();
        } catch (\Throwable $e) {
            yield $saga->compensate();
            throw $e;
        }
    }
}
