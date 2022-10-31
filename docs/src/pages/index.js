import React from 'react';
import useBaseUrl from "@docusaurus/useBaseUrl";

import styles from './index.module.css';

export default function Home() {
  return <Redirect to={useBaseUrl("/getting-started/introduction")} />;
}
