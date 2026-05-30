// Copyright (c) glassbox Authors.
// SPDX-License-Identifier: Apache-2.0

// Example usage of generated TypeScript bindings.
// Run `glassbox generate-bindings contract.wasm --output ./generated` first.

import * as StellarSdk from '@stellar/stellar-sdk';

// ── Imports from generated bindings ──────────────────────────────────────────
// import { MyContractClient }  from './generated/client';
// import { CONTRACT_METADATA } from './generated/metadata';
// import { ErstSimulator }     from './generated/Glassbox-integration';
// import { TokenError, TokenErrorError } from './generated/types';

async function main() {
  const contractId = 'CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQAHHAGCN4B2';
  const sourceKeypair = StellarSdk.Keypair.fromSecret(
    'SXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX',
  );

  // ── Example 1: Basic typed call ───────────────────────────────────────────
  console.log('\n=== Example 1: Typed contract call ===');
  // const client = new MyContractClient({ contractId, network: 'testnet' });
  // const result = await client.transfer(
  //   sourceKeypair,
  //   'GDQP2KPQGKIHYJGXNUIYOMHARUARCA7DJT5FO2FFOOKY3B2WSQHG4W37', // from: Address
  //   'GBBD47IF6LWK7P7MDEVSCWR7DPUWV3NY3DTQEVFL4NAT4AQH3ZLLFLA5', // to: Address
  //   BigInt(1_000_000),                                             // amount: bigint
  // );
  // console.log('tx hash:', result.transactionHash);

  // ── Example 2: Simulation ─────────────────────────────────────────────────
  console.log('\n=== Example 2: Simulation ===');
  // const simResult = await client.transfer(
  //   sourceKeypair, from, to, BigInt(1_000_000),
  //   { simulate: true },
  // );
  // console.log('status:', simResult.simulation?.status);
  // console.log('CPU %:', simResult.simulation?.budgetUsage?.cpuUsagePercent);

  // ── Example 3: Debug metadata ─────────────────────────────────────────────
  console.log('\n=== Example 3: Debug metadata ===');
  // const debugResult = await client.transfer(
  //   sourceKeypair, from, to, BigInt(1_000_000),
  //   { withDebugMetadata: true, simulate: true },
  // );
  // console.log('function name:', debugResult.debugMetadata?.name);
  // console.log('inputs:', debugResult.debugMetadata?.inputs);
  // console.log('source:', debugResult.debugMetadata?.source);
  //
  // Access the metadata registry directly:
  // for (const [name, meta] of Object.entries(CONTRACT_METADATA)) {
  //   console.log(`${name}: ${meta.inputs.length} inputs → ${meta.outputs.join(', ')}`);
  // }

  // ── Example 4: Debug a failed transaction ─────────────────────────────────
  console.log('\n=== Example 4: Debug failed transaction ===');
  // const simulator = new ErstSimulator({
  //   network: 'testnet',
  //   rpcUrl: 'https://soroban-testnet.stellar.org',
  // });
  // const failedResult = await simulator.debugTransaction(
  //   '5c0a1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab',
  // );
  // console.log('error:', failedResult.error);
  // console.log('events:', failedResult.events);

  // ── Example 5: Typed error handling ──────────────────────────────────────
  console.log('\n=== Example 5: Typed error handling ===');
  // try {
  //   await client.transfer(sourceKeypair, from, to, BigInt(999_999_999_999));
  // } catch (e) {
  //   if (e instanceof TokenErrorError) {
  //     switch (e.code) {
  //       case TokenError.InsufficientBalance:
  //         console.error('Not enough tokens');
  //         break;
  //       case TokenError.Unauthorized:
  //         console.error('Missing authorization');
  //         break;
  //     }
  //   }
  // }

  console.log('\nDone. Uncomment the examples above after running generate-bindings.');
}

main().catch(console.error);
