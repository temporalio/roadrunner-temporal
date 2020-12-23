<?php


namespace Temporal\Tests\Workflow;

use React\Promise\Deferred;
use React\Promise\PromiseInterface;
use Temporal\Client\Workflow;
use Temporal\Client\Workflow\WorkflowMethod;
use function Symfony\Component\String\s;

class WorkflowWithSignalledLoop
{
    #[WorkflowMethod(name: 'WorkflowWithSignalledLoop')]
    public function handler()
    {
        // tick on every signal until exit signal
        $counter = 0;
        while (true) {
            // todo: add to Workflow
            $value = yield $this->waitSignal('tick');
            error_log(print_r($value, true));

            // todo: fix it
            if ($value === ['exit']) {
                break;
            } else {
                $counter++;
            }
        }

        return $counter;
    }

    private function waitSignal(string $name): PromiseInterface
    {
        $signal = new Deferred();
        Workflow::registerSignal($name, [$signal, 'resolve']);

        return $signal->promise();
    }
}