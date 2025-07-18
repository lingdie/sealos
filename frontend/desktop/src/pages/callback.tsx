import { useRouter } from 'next/router';
import { useEffect } from 'react';
import useSessionStore from '@/stores/session';
import { ApiResp } from '@/types';
import { Flex, Spinner } from '@chakra-ui/react';
import { isString } from 'lodash';
import { bindRequest, getRegionToken, signInRequest, unBindRequest } from '@/api/auth';
import { getAdClickData, getInviterId, getUserSemData, sessionConfig } from '@/utils/sessionConfig';
import useCallbackStore, { MergeUserStatus } from '@/stores/callback';
import { ProviderType } from 'prisma/global/generated/client';
import request from '@/services/request';
import { BIND_STATUS } from '@/types/response/bind';
import { MERGE_USER_READY } from '@/types/response/utils';
import { AxiosError, HttpStatusCode } from 'axios';
import { gtmLoginSuccess } from '@/utils/gtm';

export default function Callback() {
  const router = useRouter();
  const setProvider = useSessionStore((s) => s.setProvider);
  const setToken = useSessionStore((s) => s.setToken);
  const provider = useSessionStore((s) => s.provider);
  const compareState = useSessionStore((s) => s.compareState);
  const { setMergeUserData, setMergeUserStatus } = useCallbackStore();
  useEffect(() => {
    if (!router.isReady) return;
    let isProxy: boolean = false;
    (async () => {
      try {
        if (!provider || !['GITHUB', 'WECHAT', 'GOOGLE', 'OAUTH2'].includes(provider))
          throw new Error('provider error');
        const { code, state } = router.query;
        if (!isString(code) || !isString(state)) throw new Error('failed to get code and state');
        const compareResult = compareState(state);
        if (!compareResult.isSuccess) throw new Error('invalid state');
        if (compareResult.action === 'PROXY') {
          // proxy oauth2.0, PROXY_URL_[ACTION]_STATE
          const [_url, ...ret] = compareResult.statePayload;
          await new Promise<URL>((resolve, reject) => {
            resolve(new URL(decodeURIComponent(_url)));
          })
            .then(async (url) => {
              const result = (await request(`/api/auth/canProxy?domain=${url.host}`)) as ApiResp<{
                containDomain: boolean;
              }>;
              isProxy = true;
              if (result.data?.containDomain) {
                url.searchParams.append('code', code);
                url.searchParams.append('state', ret.join('_'));
                await router.replace(url.toString());
              }
            })
            .catch(() => {
              Promise.resolve();
            });
          if (isProxy) {
            // prevent once token
            setProvider();
            isProxy = false;
            return;
          }
        } else {
          const { statePayload, action } = compareResult;
          // return
          if (action === 'LOGIN') {
            const data = await signInRequest(provider)({
              code,
              inviterId: getInviterId() ?? undefined,
              semData: getUserSemData() ?? undefined,
              adClickData: getAdClickData() ?? undefined
            });
            setProvider();
            if (data.code === 200 && data.data?.token) {
              const token = data.data?.token;
              setToken(token);
              const needInit = data.data.needInit;

              if (needInit) {
                gtmLoginSuccess({
                  user_type: 'new',
                  method: 'oauth2',
                  oauth2Provider: provider
                });
                await router.push('/workspace');
                return;
              }
              gtmLoginSuccess({
                user_type: 'existing',
                method: 'oauth2',
                oauth2Provider: provider
              });
              const regionTokenRes = await getRegionToken();
              if (regionTokenRes?.data) {
                await sessionConfig(regionTokenRes.data);
                await router.replace('/');
              }
            } else {
              throw new Error();
            }
          } else if (action === 'BIND') {
            try {
              const response = await bindRequest(provider)({ code });
              if (response.message === BIND_STATUS.RESULT_SUCCESS) {
                setProvider();
                await router.replace('/');
              } else if (response.message === MERGE_USER_READY.MERGE_USER_CONTINUE) {
                const code = response.data?.code;
                if (!code) return;
                setMergeUserData({
                  providerType: provider as ProviderType,
                  code
                });
                setMergeUserStatus(MergeUserStatus.CANMERGE);
                setProvider();
                await router.replace('/');
              } else if (response.message === MERGE_USER_READY.MERGE_USER_PROVIDER_CONFLICT) {
                setMergeUserData();
                setMergeUserStatus(MergeUserStatus.CONFLICT);
                setProvider();
                await router.replace('/');
              }
            } catch (bindError) {
              if ((bindError as any)?.message === MERGE_USER_READY.MERGE_USER_PROVIDER_CONFLICT) {
                setMergeUserData();
                setMergeUserStatus(MergeUserStatus.CONFLICT);
                setProvider();
                await router.replace('/');
              } else {
                console.log('unkownerror', bindError);
                throw Error();
              }
            }
          } else if (action === 'UNBIND') {
            await unBindRequest(provider)({ code });
            setProvider();
            await router.replace('/');
          }
        }
      } catch (error) {
        console.error(error);
        await router.replace('/signin');
      }
    })();
  }, [router]);
  return (
    <Flex w={'full'} h={'full'} justify={'center'} align={'center'}>
      <Spinner size="xl" />
    </Flex>
  );
}
// 所有含动态数据的页面（如/callback）
export async function getServerSideProps() {
  return { props: {} };
}
