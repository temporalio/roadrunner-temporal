<?php

declare(strict_types=1);

use Temporal\Client\GRPC\ServiceClient;
use Temporal\Client\WorkflowClient;
use Temporal\Tests\Nexus\GreetingHandlerWorkflow;
use Temporal\Tests\Nexus\GreetingService;
use Temporal\Tests\Nexus\NexusCallerWorkflow;
use Temporal\WorkerFactory;

require __DIR__ . '/vendor/autoload.php';

$factory = WorkerFactory::create(client: WorkflowClient::create(ServiceClient::create('127.0.0.1:7233')));
$worker = $factory->newWorker('nexus');

$worker->registerWorkflowTypes(NexusCallerWorkflow::class, GreetingHandlerWorkflow::class);
$worker->registerNexusServiceImplementation(new GreetingService());

$factory->run();
