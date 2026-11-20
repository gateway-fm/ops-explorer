import { DetailHeroSkeleton, DetailRowsSkeleton } from '../components/DetailSkeletons';
import { CoinBalancePanel } from '../components/AddressPresentation';
import { useNativeMetadata } from '../hooks/useNativeMetadata';
import { nativeTokenPath, NativeNames } from '../components/NativeNames';
import { useCanonicalIdentifier } from '../lib/nativeRoute';
import { NativePortfolio } from './TokensNative';
import { NativeContractPanel } from '../components/NativeContractPanel';
import { Link, useLocation, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { AddressBackButton, AddressHero, AddressTabs, AddressDetailsPanel, PanelShell, EmptyRow, type AddressTab } from '../components/AddressPresentation';
import { NativeAddress } from '../components/NativeAddress';
import { NativeContinuation, NativeLogCards, NativeReadState, NativeTraceRows, NativeTransactionsTable, NativeTransferRows, useNativePage } from '../components/NativeReadPanels';
import { useNativeRead } from '../hooks/useNativeRead';
import { decodeToken, decodeAddressStats, decodeBalance, decodeCode, decodeContract, decodeTransactions, decodeTransfers, decodeLogs, decodeInternalTransactions } from '../lib/nativeReadModels';
import { exactQuantity, exactUnits, exactTimestamp } from '../lib/nativePresentation';
import { getNetworkCurrency } from '../lib/utils';
import { StateMessage } from '../components/StateMessage';

const suffixTab: Record<string, AddressTab> = {stats:'details',transactions:'transactions',transfers:'transfers',balances:'tokens',internal:'internal',balance:'coin-history',logs:'logs',code:'contract',contract:'contract','is-contract':'details'};
const tabs: AddressTab[] = ['details','transactions','transfers','tokens','internal','coin-history','logs','contract'];
export function AddressNative() {
  const {address: rawIdentifier = ''} = useParams<{address:string}>();
  const address = useCanonicalIdentifier(rawIdentifier, 'address') ?? '';
  const navigate = useNavigate(); const location = useLocation(); const [params] = useSearchParams();
  const selected = params.get('tab') || suffixTab[location.pathname.split('/').at(-1) || ''];
  const active: AddressTab = tabs.includes(selected as AddressTab) ? selected as AddressTab : 'details';
  const valid = /^0x[0-9a-f]{40}$/.test(address) && address !== `0x${'0'.repeat(40)}`;
  const path = valid ? `/addresses/${address}` : null;
  const info = useNativeRead(path && `/v2/explorer${path}/stats`, decodeAddressStats);
  const balance = useNativeRead(path && (active === 'details' || active === 'coin-history') ? `/v2/explorer${path}/balance` : null, decodeBalance);
  const code = useNativeRead(path && `/v2/explorer${path}/code`, decodeCode);
  const contract = useNativeRead(path && `/v2/explorer${path}/contract`, decodeContract);
  const heroTokens = useNativeMetadata(valid ? [nativeTokenPath(address)] : [], decodeToken, 2);
  const heroToken = heroTokens[nativeTokenPath(address)];
  const txs = useNativePage(path && `${path}/transactions`, decodeTransactions, 25, '', active === 'transactions');
  const transfers = useNativePage(path && `${path}/transfers`, decodeTransfers, 25, '', active === 'transfers');
  const logs = useNativePage(path && `${path}/logs`, decodeLogs, 25, '', active === 'logs');
  const internal = useNativePage(path && `${path}/internal`, decodeInternalTransactions, 25, '', active === 'internal');
  if (!valid) return <StateMessage variant="error" title="Invalid or private address"/>;
  if (info.loading) return <div className="space-y-6"><AddressBackButton onClick={() => navigate(-1)}/><DetailHeroSkeleton/></div>;
  if (info.error) return <div className="space-y-6"><AddressBackButton onClick={() => navigate(-1)}/><NativeReadState {...info}/></div>;
  const stats = info.data ?? {address};
  const isContract = stats.isContract ?? (code.data ? code.data.code !== '0x' : undefined);
  const setTab = (tab: AddressTab) => { const query=new URLSearchParams(params); if(tab==='details')query.delete('tab');else query.set('tab',tab); navigate({pathname:`/address/${address}`,search:query.toString(),hash:location.hash},{state:location.state}); };
  return <NativeNames items={contract.data?.creator?[{address:contract.data.creator}]:[]}><div className="space-y-6">
    <AddressBackButton onClick={() => navigate(-1)}/><AddressHero address={address} isContract={isContract} contract={contract.data ?? null} token={heroToken?.address === address ? heroToken : null}/>
    <AddressTabs active={active} onChange={setTab} isContract={isContract || active === 'contract'} txCount={stats.txCount === undefined ? undefined : exactQuantity(stats.txCount)} tokenTransferCount={stats.tokenTransferCount === undefined ? undefined : exactQuantity(stats.tokenTransferCount)} internalTxCount={stats.internalTxCount === undefined ? undefined : exactQuantity(stats.internalTxCount)}/>
    <section>
      <NativeReadState {...info}/>
      {active === 'details' && <><NativeReadState {...balance}/><AddressDetailsPanel rows={[
        {label:'Balance',value:<span className="font-mono tabular-nums">{balance.data ? `${exactUnits(balance.data.balance)} ${getNetworkCurrency()}` : 'Unavailable'}</span>},
        {label:'Transactions',value:exactQuantity(stats.txCount)}, {label:'Token transfers',value:exactQuantity(stats.tokenTransferCount)}, {label:'Internal txns',value:exactQuantity(stats.internalTxCount)},
        ...(stats.firstSeen === undefined ? [] : [{label:'First seen',value:exactTimestamp(stats.firstSeen)}]), ...(stats.lastSeen === undefined ? [] : [{label:'Last seen',value:exactTimestamp(stats.lastSeen)}]),
        ...(contract.data?.creator ? [{label:'Creator',value:<NativeAddress address={contract.data.creator}/>}]:[]),
        ...(contract.data?.creationTx ? [{label:'Creation transaction',value:<Link className="font-mono text-primary break-all" to={`/tx/${contract.data.creationTx}`}>{contract.data.creationTx}</Link>}]:[]),
      ]}/><p className="mt-3 text-xs text-neutral-500">Counts are shown only when supplied for your current access. Unavailable is not zero.</p></>}
      {active === 'transactions' && <><NativeReadState {...txs} reset={txs.reset} loadingView={<DetailRowsSkeleton/>}/>{txs.data && <><NativeTransactionsTable items={txs.data.items} currentAddress={address} footer={<NativeContinuation {...txs} page={txs.data.page}/>}/>{!txs.data.items.length && <EmptyRow>No direct transactions on this visible page. Continue if more records are available.</EmptyRow>}</>}</>}
      {active === 'transfers' && <><NativeReadState {...transfers} reset={transfers.reset} loadingView={<DetailRowsSkeleton/>}/>{transfers.data && <PanelShell><NativeTransferRows items={transfers.data.items} currentAddress={address}/><NativeContinuation {...transfers} page={transfers.data.page}/></PanelShell>}</>}
      {active === 'logs' && <><NativeReadState {...logs} reset={logs.reset} loadingView={<DetailRowsSkeleton rows={3}/>}/>{logs.data && <><NativeLogCards items={logs.data.items}/><NativeContinuation {...logs} page={logs.data.page}/></>}</>}
      {active === 'internal' && <><NativeReadState {...internal} reset={internal.reset} loadingView={<DetailRowsSkeleton rows={3}/>}/>{internal.data && <><NativeTraceRows items={internal.data.items}/><NativeContinuation {...internal} page={internal.data.page}/></>}</>}
      {active === 'coin-history' && <><NativeReadState {...balance}/><CoinBalancePanel balance={balance.data ? `${exactUnits(balance.data.balance)} ${getNetworkCurrency()}` : 'Unavailable'} observedBlock={balance.data?.blockNumber !== undefined ? <Link className="text-primary" to={`/block/${balance.data.blockNumber}`}>{exactQuantity(balance.data.blockNumber)}</Link> : undefined}/></>}
      {active === 'tokens' && <NativePortfolio key={address} address={address}/>}
      {active === 'contract' && <div className="space-y-4"><NativeReadState {...code}/><NativeReadState {...contract}/><NativeContractPanel key={address} address={address} contract={contract.data} code={code.data?.code}/></div>}
    </section>
  </div></NativeNames>;
}
