// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

const path = require('path');
const grpc = require('grpc');
const pino = require('pino');
const protoLoader = require('@grpc/proto-loader');


const oc = require('@opencensus/nodejs');
const { plugin } = require('@opencensus/instrumentation-grpc');
const { JaegerTraceExporter } = require('@opencensus/exporter-jaeger');
const charge = require('./charge');

const { JAEGER_AGENT_HOST } = process.env;
const { JAEGER_AGENT_PORT } = process.env;

const logger = pino({
  name: 'paymentservice-server',
  messageKey: 'message',
  changeLevelName: 'severity',
  useLevelLabels: true,
});

logger.info(`JAEGER_AGENT_HOST: ${JAEGER_AGENT_HOST}`);
logger.info(`JAEGER_AGENT_PORT: ${JAEGER_AGENT_PORT}`);

const jaegerOptions = {
  serviceName: 'paymentservice',
  host: JAEGER_AGENT_HOST,
  port: JAEGER_AGENT_PORT,
  bufferTimeout: 10, // time in milliseconds
};

const exporter = new JaegerTraceExporter(jaegerOptions);
const tracing = oc.registerExporter(exporter).start();

const { tracer } = tracing.start({
  samplingRate: 1, // For demo purposes, always sample
});

// Enables GRPC plugin: Method that enables the instrumentation patch.
plugin.enable(grpc, tracer, '^1.22.2', {});

class HipsterShopServer {
  constructor(protoRoot, port = HipsterShopServer.PORT) {
    this.port = port;

    this.packages = {
      hipsterShop: this.loadProto(path.join(protoRoot, 'demo.proto')),
      health: this.loadProto(path.join(protoRoot, 'grpc/health/v1/health.proto')),
    };

    this.server = new grpc.Server();
    this.loadAllProtos();
  }

  /**
   * Handler for PaymentService.Charge.
   * @param {*} call  { ChargeRequest }
   * @param {*} callback  fn(err, ChargeResponse)
   */
  static ChargeServiceHandler(call, callback) {
    try {
      logger.info(`PaymentService#Charge invoked with request ${JSON.stringify(call.request)}`);
      const response = charge(call.request);
      callback(null, response);
    } catch (err) {
      console.warn(err);
      callback(err);
    }
  }

  static CheckHandler(call, callback) {
    callback(null, { status: 'SERVING' });
  }

  listen() {
    this.server.bind(`0.0.0.0:${this.port}`, grpc.ServerCredentials.createInsecure());
    logger.info(`PaymentService grpc server listening on ${this.port}`);
    this.server.start();
  }


  static loadProto(protoPath) {
    const packageDefinition = protoLoader.loadSync(
      protoPath,
      {
        keepCase: true,
        longs: String,
        enums: String,
        defaults: true,
        oneofs: true,
      },
    );
    return grpc.loadPackageDefinition(packageDefinition);
  }

  loadAllProtos() {
    const hipsterShopPackage = this.packages.hipsterShop.hipstershop;
    const healthPackage = this.packages.health.grpc.health.v1;

    this.server.addService(
      hipsterShopPackage.PaymentService.service,
      {
        charge: HipsterShopServer.ChargeServiceHandler.bind(this),
      },
    );

    this.server.addService(
      healthPackage.Health.service,
      {
        check: HipsterShopServer.CheckHandler.bind(this),
      },
    );
  }
}

HipsterShopServer.PORT = process.env.PORT;

module.exports = HipsterShopServer;
